// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package uploaders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/http2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RedHatInsightsUploaderConfig struct {
	URL                 string   `json:"url"`
	Token               string   `json:"-"`
	OperatorVersion     string   `json:"operatorVersion"`
	ClusterID           string   `json:"clusterID"`
	AdditionalCertFiles []string `json:"additionalCertFiles,omitempty"`
	httpVersion         *int
}

type RedHatInsightsUploader struct {
	RedHatInsightsUploaderConfig
	client *http.Client
}

var _ Uploader = &RedHatInsightsUploader{}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

const mktplaceFileUploadType = "application/vnd.redhat.mkt.tar+tgz"
const insightsUserAgentFmt = "marketplace-operator/%s cluster/%s"
const insightsUploadURL = "%s/api/ingress/v1/upload"

func getUserAgent(version, clusterID string) string {
	return fmt.Sprintf(insightsUserAgentFmt, version, clusterID)
}

func NewRedHatInsightsUploader(
	config *RedHatInsightsUploaderConfig,
) (Uploader, error) {
	tlsConfig, err := prometheus.GenerateCACertPool(config.AdditionalCertFiles...)

	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	config.httpVersion = ptr.Int(1)

	// default to 2 unless otherwise overridden
	if config.httpVersion == nil {
		config.httpVersion = ptr.Int(2)
	}

	proxyCfg := httpproxy.FromEnvironment()
	if proxyCfg.HTTPProxy != "" || proxyCfg.HTTPSProxy != "" {
		config.httpVersion = ptr.Int(1)
	}

	// Use the proper transport in the client
	switch *config.httpVersion {
	case 1:
		client.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
		}
	case 2:
		client.Transport = &http2.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	client.Transport = WithBearerAuth(client.Transport, config.Token)

	return &RedHatInsightsUploader{
		client:                       client,
		RedHatInsightsUploaderConfig: *config,
	}, nil
}

func ProvideRedHatInsightsUploader(
	ctx context.Context,
	client client.Client,
	log logr.Logger,
) (Uploader, error) {

	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{
		Name:      "pull-secret",
		Namespace: "openshift-config",
	}, secret); err != nil {
		return nil, err
	}

	clusterVersion := &openshiftconfigv1.ClusterVersion{}
	if err := client.Get(ctx, types.NamespacedName{
		Name: "version",
	}, clusterVersion); err != nil {
		return nil, err
	}

	dockerConfigBytes, ok := secret.Data[".dockerconfigjson"]

	if !ok {
		return nil, errors.WithStack(&ReportJobError{
			ErrorMessage: "failed to find .dockerconfigjson in secret openshift-config/pull-secret",
			Err:          errors.New("report job error"),
		})
	}

	var dockerObj interface{}
	err := json.Unmarshal(dockerConfigBytes, &dockerObj)

	if err != nil {
		return nil, errors.WithStack(&ReportJobError{
			ErrorMessage: "failed to unmarshal .dockerconfigjson from secret openshift-config/pull-secret",
			Err:          err,
		})
	}

	cloudAuthPath := jsonpath.New("cloudauthpath")
	err = cloudAuthPath.Parse(`{.auths.cloud\.openshift\.com.auth}`)

	if err != nil {
		return nil, errors.WithStack(&ReportJobError{
			ErrorMessage: "failed to parse auth token in .dockerconfigjson from secret openshift-config/pull-secret",
			Err:          err,
		})
	}

	buf := new(bytes.Buffer)
	err = cloudAuthPath.Execute(buf, dockerObj)

	if err != nil {
		return nil, errors.WithStack(&ReportJobError{
			ErrorMessage: "failed to template auth token in .dockerconfigjson from secret openshift-config/pull-secret",
			Err:          err,
		})
	}

	cloudToken := buf.String()

	return NewRedHatInsightsUploader(&RedHatInsightsUploaderConfig{
		URL:             "https://cloud.redhat.com",
		ClusterID:       string(clusterVersion.Spec.ClusterID), // get from cluster
		OperatorVersion: version.Version,
		Token:           cloudToken, // get from secret
	})
}

func (r *RedHatInsightsUploader) uploadFileRequest(fileName string, reader io.Reader) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes("file"), escapeQuotes(filepath.Base(fileName))))
	h.Set("Content-Type", mktplaceFileUploadType)
	part, err := writer.CreatePart(h)

	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, reader)

	if err != nil {
		return nil, err
	}

	_ = writer.WriteField("type", mktplaceFileUploadType)

	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf(insightsUploadURL, r.URL), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", getUserAgent(r.OperatorVersion, r.ClusterID))
	return req, err
}

func (r *RedHatInsightsUploader) UploadFile(ctx context.Context, fileName string, reader io.Reader) (string, error) {
	req, err := r.uploadFileRequest(fileName, reader)

	if err != nil {
		return "", errors.Wrap(err, "failed to get upload file req")
	}

	// Perform the request
	resp, err := r.client.Do(req)
	if err != nil {
		logger.Error(err, "failed to post")
		return "", errors.Wrap(err, "failed to post")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}

	logger.Info(
		"retrieved response",
		"statusCode", resp.StatusCode,
		"proto", resp.Proto,
		"body", string(body),
		"headers", resp.Header)

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return "", errors.NewWithDetails("failed to upload field",
			"statusCode", resp.StatusCode,
			"proto", resp.Proto,
			"body", string(body),
			"headers", resp.Header)
	}

	data := map[string]interface{}{}
	err = json.Unmarshal(body, &data)

	if err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}

	// todo get the id
	return "", nil
}
