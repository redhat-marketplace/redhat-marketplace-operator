// Copyright 2020 IBM Corp.
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

package reporter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/jsonpath"
)

type UploaderTarget interface {
	Name() string
}

var (
	UploaderTargetRedHatInsights UploaderTarget = &RedHatInsightsUploader{}
	UploaderTargetNoOp           UploaderTarget = &NoOpUploader{}
	UploaderTargetLocalPath      UploaderTarget = &LocalFilePathUploader{}
	UploaderTargetAirGap  		 UploaderTarget = &AirGapUploader{}
)

func (u *AirGapUploader) Name() string {
	return "air-gap"
}

func (u *RedHatInsightsUploader) Name() string {
	return "redhat-insights"
}

func (u *NoOpUploader) Name() string {
	return "noop"
}

func (u *LocalFilePathUploader) Name() string {
	return "local-path"
}

func MustParseUploaderTarget(s string) UploaderTarget {
	switch s {
	case UploaderTargetRedHatInsights.Name():
		return UploaderTargetRedHatInsights
	case UploaderTargetLocalPath.Name():
		return UploaderTargetLocalPath
	case UploaderTargetNoOp.Name():
		return UploaderTargetNoOp
	case UploaderTargetAirGap.Name():
		return UploaderTargetAirGap
	default:
		panic(errors.Errorf("provided string is not a valid upload target %s", s))
	}
}

type Uploader interface {
	UploadFile(path string) error
}

type AirGapUploader struct {
	AirGapUploaderConfig
	// client filesender.FileSenderClient
	client filesender.FileSender_UploadFileClient
}

type AirGapUploaderConfig struct {
	URL    string `json:"url"`
	FilePath string `json:"filePath"`
}

func NewAirGapUploader (config *AirGapUploaderConfig )(Uploader, error){
	//Create socket connection
	conn, err := grpc.Dial(config.URL, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	//Create uplaod client
	client := filesender.NewFileSenderClient(conn)

	//client-stream upload the file
	uploadClient, err := client.UploadFile(context.Background())
	if err != nil {
		// log.Fatalf("While calling uploadFile: %v", err)
		return nil, err
	}

	return &AirGapUploader{
		AirGapUploaderConfig: *config,
		client: uploadClient,
	}, nil
}

func (a *AirGapUploader) UploadFile(path string) error {
	//Create socket connection
	var address  = "rhm-dqlite.openshift-redhat-marketplace.svc.local" //TODO: get address of the ingress for the data services
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		// log.Fatalf("did not connect: %v", err)
		logger.Error(err,"failed to establish connection")
		return err
	}

	defer conn.Close()

	m := map[string]string{
		"version":    "v1",
		"reportType": "rhm-metering",
	}

	chunkAndUpload(a.client, path, m)

	return nil
	
}

func chunkAndUpload(uploadClient filesender.FileSender_UploadFileClient, path string, m map[string]string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer func() error {
		if err := file.Close(); err != nil {
			return err
		}

		return nil
	}()

	//Getting file metadata
	metaData, err := file.Stat()
	if err != nil {
		logger.Error(err,"Failed to get metadata")
		return err
		// log.Fatalf("Failed to get metadata: %v", err)
	}

	uploadClient.Send(&filesender.UploadFileRequest{
		Data: &filesender.UploadFileRequest_Info{
			Info: &v1.FileInfo{
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: metaData.Name(),
					},
				},
				Size:     uint32(metaData.Size()),
				Metadata: m,
			},
		},
	})

	//Chunking
	chunkSize := 3
	buffReader := bufio.NewReader(file)
	buffer := make([]byte, chunkSize)
	for {
		n, err := buffReader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				// fmt.Printf("Error reading file: %v", err)
				logger.Error(err,"Error reading file")
			}
			break
		}
		//Sending request
		request := filesender.UploadFileRequest{
			Data: &filesender.UploadFileRequest_ChunkData{
				ChunkData: buffer[0:n],
			},
		}
		uploadClient.Send(&request)
	}

	res, err := uploadClient.CloseAndRecv()
	if err != nil {
		logger.Error(err,"Error getting response")
	}

	// fmt.Println(res)
	logger.Info("airgap upload","response",res)

	return nil

}

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

func NewRedHatInsightsUploader(
	config *RedHatInsightsUploaderConfig,
) (Uploader, error) {
	tlsConfig, err := prometheus.GenerateCACertPool(config.AdditionalCertFiles...)

	if err != nil {
		return nil, err
	}

	client := &http.Client{}

	// default to 2 unless otherwise overridden
	if config.httpVersion == nil {
		config.httpVersion = ptr.Int(2)
	}

	// Use the proper transport in the client
	switch *config.httpVersion {
	case 1:
		client.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	case 2:
		client.Transport = &http2.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	return &RedHatInsightsUploader{
		client:                       client,
		RedHatInsightsUploaderConfig: *config,
	}, nil
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

const mktplaceFileUploadType = "application/vnd.redhat.mkt.tar+tgz"
const userAgentFmt = "marketplace-operator/%s cluster/%s"

const uploadURL = "%s/api/ingress/v1/upload"

func getUserAgent(version, clusterID string) string {
	return fmt.Sprintf(userAgentFmt, version, clusterID)
}

func (r *RedHatInsightsUploader) uploadFileRequest(path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes("file"), escapeQuotes(filepath.Base(path))))
	h.Set("Content-Type", mktplaceFileUploadType)
	part, err := writer.CreatePart(h)

	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, file)

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

	req, err := http.NewRequest("POST", fmt.Sprintf(uploadURL, r.URL), body)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.Token))
	req.Header.Set("User-Agent", getUserAgent(r.OperatorVersion, r.ClusterID))
	return req, err
}

func (r *RedHatInsightsUploader) UploadFile(path string) error {
	req, err := r.uploadFileRequest(path)

	if err != nil {
		return errors.Wrap(err, "failed to get upload file req")
	}

	// Perform the request
	resp, err := r.client.Do(req)
	if err != nil {
		logger.Error(err, "failed to post")
		return errors.Wrap(err, "failed to post")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	logger.Info(
		"retrieved response",
		"statusCode", resp.StatusCode,
		"proto", resp.Proto,
		"body", string(body),
		"headers", resp.Header)

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return errors.NewWithDetails("failed to upload field",
			"statusCode", resp.StatusCode,
			"proto", resp.Proto,
			"body", string(body),
			"headers", resp.Header)
	}
	return nil
}

type NoOpUploader struct{}

var _ Uploader = &NoOpUploader{}

func (r *NoOpUploader) UploadFile(path string) error {
	log := logger.WithValues("uploader", "noop")
	log.Info("upload is a no op")
	return nil
}

type LocalFilePathUploader struct {
	LocalFilePath string
}

func (r *LocalFilePathUploader) UploadFile(path string) error {
	if _, err := os.Stat(r.LocalFilePath); err != nil {
		return err
	}

	if r.LocalFilePath == "" {
		r.LocalFilePath = "."
	}

	log := logger.WithValues("uploader", "localFilePath")

	baseName := filepath.Base(path)
	fileName := filepath.Join(r.LocalFilePath, baseName)

	input, err := ioutil.ReadFile(path)
	if err != nil {
		log.Error(err, "Error opening input", "name", path)
		return err
	}

	log.Info("creating file", "name", fileName)
	err = ioutil.WriteFile(fileName, input, 0644)
	if err != nil {
		log.Error(err, "Error creating", "name", fileName)
		return err
	}

	return nil
}

func ProvideUploader(
	ctx context.Context,
	cc ClientCommandRunner,
	log logr.Logger,
	uploaderTarget UploaderTarget,
) (Uploader, error) {
	switch uploaderTarget.(type) {
	case *RedHatInsightsUploader:
		config, err := provideProductionInsightsConfig(ctx, cc, log)

		if err != nil {
			return nil, err
		}

		return NewRedHatInsightsUploader(config)
	case *NoOpUploader:
		return uploaderTarget.(Uploader), nil
	case *LocalFilePathUploader:
		return uploaderTarget.(Uploader), nil
	case *AirGapUploader:
		return uploaderTarget.(Uploader),nil
	}

	return nil, errors.Errorf("uploader target not available %s", uploaderTarget.Name())
}

func provideProductionInsightsConfig(
	ctx context.Context,
	cc ClientCommandRunner,
	log logr.Logger,
) (*RedHatInsightsUploaderConfig, error) {
	secret := &corev1.Secret{}
	clusterVersion := &openshiftconfigv1.ClusterVersion{}
	result, _ := cc.Do(ctx,
		GetAction(types.NamespacedName{
			Name:      "pull-secret",
			Namespace: "openshift-config",
		}, secret),
		GetAction(types.NamespacedName{
			Name: "version",
		}, clusterVersion))

	if !result.Is(Continue) {
		return nil, result
	}

	dockerConfigBytes, ok := secret.Data[".dockerconfigjson"]

	if !ok {
		return nil, errors.New(".dockerconfigjson is not found in secret")
	}

	var dockerObj interface{}
	err := json.Unmarshal(dockerConfigBytes, &dockerObj)

	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal dockerConfigJson object")
	}

	cloudAuthPath := jsonpath.New("cloudauthpath")
	err = cloudAuthPath.Parse(`{.auths.cloud\.openshift\.com.auth}`)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get jsonpath of cloud token")
	}

	buf := new(bytes.Buffer)
	err = cloudAuthPath.Execute(buf, dockerObj)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get jsonpath of cloud token")
	}

	cloudToken := buf.String()

	return &RedHatInsightsUploaderConfig{
		URL:             "https://cloud.redhat.com",
		ClusterID:       string(clusterVersion.Spec.ClusterID), // get from cluster
		OperatorVersion: version.Version,
		Token:           cloudToken, // get from secret
	}, nil
}
