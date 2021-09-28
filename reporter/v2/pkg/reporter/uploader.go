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

	golangErr "errors"

	"emperror.dev/errors"
	"github.com/go-logr/logr"

	"github.com/gotidy/ptr"

	openshiftconfigv1 "github.com/openshift/api/config/v1"

	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/http2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/jsonpath"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3/s3manager"

	yaml "gopkg.in/yaml.v2"
)

type UploaderTargets []UploaderTarget

type UploaderTarget interface {
	Name() string
}

var (
	UploaderTargetRedHatInsights UploaderTarget = &RedHatInsightsUploader{}
	UploaderTargetNoOp           UploaderTarget = &NoOpUploader{}
	UploaderTargetLocalPath      UploaderTarget = &LocalFilePathUploader{}
	UploaderTargetDataService    UploaderTarget = &DataServiceUploader{}
	UploaderTargetCOSS3          UploaderTarget = &COSS3Uploader{}
)

func (u *DataServiceUploader) Name() string {
	return "data-service"
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

func (u *COSS3Uploader) Name() string {
	return "cos-s3"
}

func MustParseUploaderTarget(s string) UploaderTarget {
	switch s {
	case UploaderTargetRedHatInsights.Name():
		return UploaderTargetRedHatInsights
	case UploaderTargetLocalPath.Name():
		return UploaderTargetLocalPath
	case UploaderTargetNoOp.Name():
		return UploaderTargetNoOp
	case UploaderTargetDataService.Name():
		return UploaderTargetDataService
	case UploaderTargetCOSS3.Name():
		return UploaderTargetCOSS3
	default:
		panic(errors.Errorf("provided string is not a valid upload target %s", s))
	}
}

type Uploaders []Uploader

type Uploader interface {
	UploaderTarget
	UploadFile(path string) error
}

type DataServiceUploader struct {
	Ctx context.Context
	DataServiceConfig
	UploadClient filesender.FileSender_UploadFileClient
}

type DataServiceConfig struct {
	Address          string `json:"address"`
	DataServiceToken string `json:"dataServiceToken"`
	DataServiceCert  []byte `json:"dataServiceCert"`
}

func provideDataServiceConfig(deployedNamespace string, dataServiceTokenFile string, dataServiceCertFile string) (*DataServiceConfig, error) {
	cert, err := ioutil.ReadFile(dataServiceCertFile)
	if err != nil {
		return nil, err
	}

	var serviceAccountToken = ""
	if dataServiceTokenFile != "" {
		content, err := ioutil.ReadFile(dataServiceTokenFile)
		if err != nil {
			return nil, err
		}
		serviceAccountToken = string(content)
	}

	var dataServiceDNS = fmt.Sprintf("%s.%s.svc:8004", utils.DATA_SERVICE_NAME, deployedNamespace)

	return &DataServiceConfig{
		Address:          dataServiceDNS,
		DataServiceToken: serviceAccountToken,
		DataServiceCert:  cert,
	}, nil
}

func NewDataServiceUploader(ctx context.Context, dataServiceConfig *DataServiceConfig) (Uploader, error) {
	uploadClient, err := createDataServiceUploadClient(ctx, dataServiceConfig)
	if err != nil {
		return nil, err
	}

	return &DataServiceUploader{
		UploadClient:      uploadClient,
		DataServiceConfig: *dataServiceConfig,
	}, nil
}

func createDataServiceUploadClient(ctx context.Context, dataServiceConfig *DataServiceConfig) (filesender.FileSender_UploadFileClient, error) {
	logger.Info("airgap url", "url", dataServiceConfig.Address)

	conn, err := newGRPCConn(ctx, dataServiceConfig.Address, dataServiceConfig.DataServiceCert, dataServiceConfig.DataServiceToken)

	if err != nil {
		logger.Error(err, "failed to establish connection")
		return nil, err
	}

	client := filesender.NewFileSenderClient(conn)

	uploadClient, err := client.UploadFile(context.Background())
	if err != nil {
		logger.Error(err, "could not initialize uploadClient")
		return nil, err
	}

	return uploadClient, nil
}

func (d *DataServiceUploader) UploadFile(path string) error {
	m := map[string]string{
		"version":    "v1",
		"reportType": "rhm-metering",
	}

	logger.Info("starting chunk and upload", "file name", path)
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

	metaData, err := file.Stat()
	if err != nil {
		logger.Error(err, "Failed to get metadata")
		return err
	}

	err = d.UploadClient.Send(&filesender.UploadFileRequest{
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

	if err != nil {
		logger.Error(err, "Failed to create metadata UploadFile request")
		return err
	}

	chunkSize := 3
	buffReader := bufio.NewReader(file)
	buffer := make([]byte, chunkSize)
	for {
		n, err := buffReader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				logger.Error(err, "Error reading file")
			}
			break
		}

		request := filesender.UploadFileRequest{
			Data: &filesender.UploadFileRequest_ChunkData{
				ChunkData: buffer[0:n],
			},
		}
		err = d.UploadClient.Send(&request)
		if err != nil {
			logger.Error(err, "Failed to create UploadFile request")
			return err
		}
	}

	res, err := d.UploadClient.CloseAndRecv()
	if err != nil {
		if err == io.EOF {
			logger.Info("Stream EOF")
			return nil
		}

		logger.Error(err, "Error getting response")
		return err
	}

	logger.Info("airgap upload response", "response", res)

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

func ProvideUploaders(
	ctx context.Context,
	cc ClientCommandRunner,
	log logr.Logger,
	reporterConfig *Config,
) (Uploaders, error) {
	uploaders := Uploaders{}

	for _, target := range reporterConfig.UploaderTargets {
		switch target.(type) {
		case *RedHatInsightsUploader:
			config, err := provideProductionInsightsConfig(ctx, cc, log)

			if err != nil {
				return nil, err
			}

			uploader, err := NewRedHatInsightsUploader(config)
			if err != nil {
				return uploaders, err
			}
			uploaders = append(uploaders, uploader)
		case *NoOpUploader:
			uploaders = append(uploaders, target.(Uploader))
		case *LocalFilePathUploader:
			uploaders = append(uploaders, target.(Uploader))
		case *DataServiceUploader:
			dataServiceConfig, err := provideDataServiceConfig(reporterConfig.DeployedNamespace, reporterConfig.DataServiceTokenFile, reporterConfig.DataServiceCertFile)
			if err != nil {
				return nil, err
			}

			uploader, err := NewDataServiceUploader(ctx, dataServiceConfig)
			if err != nil {
				return uploaders, err
			}
			uploaders = append(uploaders, uploader)
		case *COSS3Uploader:
			cosS3Config, err := provideCOSS3Config(ctx, cc, reporterConfig.DeployedNamespace, log)
			if err != nil {
				return nil, err
			}

			uploader, err := NewCOSS3Uploader(cosS3Config)
			if err != nil {
				return uploaders, err
			}
			uploaders = append(uploaders, uploader)
		default:
			return nil, errors.Errorf("uploader target not available %s", target.Name())
		}
	}

	return uploaders, nil
}

type ReportJobError struct {
	ErrorMessage string
	Err          error
}

func (re ReportJobError) Error() string {
	return re.ErrorMessage
}

func (re ReportJobError) Unwrap() error { return re.Err }

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
		return nil, errors.Wrap(ReportJobError{
			ErrorMessage: "failed to find .dockerconfigjson in secret openshift-config/pull-secret",
			Err:          golangErr.New("report job error"),
		}, "failed to find .dockerconfigjson in secret openshift-config/pull-secret")
	}

	var dockerObj interface{}
	err := json.Unmarshal(dockerConfigBytes, &dockerObj)

	if err != nil {
		return nil, errors.Wrap(ReportJobError{
			ErrorMessage: "failed to unmarshal .dockerconfigjson from secret openshift-config/pull-secret",
			Err:          err,
		}, "failed to unmarshal .dockerconfigjson from secret openshift-config/pull-secret")
	}

	cloudAuthPath := jsonpath.New("cloudauthpath")
	err = cloudAuthPath.Parse(`{.auths.cloud\.openshift\.com.auth}`)

	if err != nil {
		return nil, errors.Wrap(ReportJobError{
			ErrorMessage: "failed to get jsonpath of cloud token",
			Err:          err,
		}, "failed to get jsonpath of cloud token")
	}

	buf := new(bytes.Buffer)
	err = cloudAuthPath.Execute(buf, dockerObj)

	if err != nil {
		return nil, errors.Wrap(ReportJobError{
			ErrorMessage: "failed to get jsonpath of cloud token",
			Err:          err,
		}, "failed to get jsonpath of cloud token")
	}

	cloudToken := buf.String()

	return &RedHatInsightsUploaderConfig{
		URL:             "https://cloud.redhat.com",
		ClusterID:       string(clusterVersion.Spec.ClusterID), // get from cluster
		OperatorVersion: version.Version,
		Token:           cloudToken, // get from secret
	}, nil
}

type COSS3UploaderConfig struct {
	ApiKey            string `json:"apiKey" yaml:"apiKey"`
	ServiceInstanceID string `json:"serviceInstanceID" yaml:"serviceInstanceID"`
	AuthEndpoint      string `json:"authEndpoint" yaml:"authEndpoint"`
	ServiceEndpoint   string `json:"serviceEndpoint" yaml:"serviceEndpoint"`
	Bucket            string `json:"bucket" yaml:"bucket"`
}

type COSS3Uploader struct {
	COSS3UploaderConfig
	uploader *s3manager.Uploader
}

var _ Uploader = &COSS3Uploader{}

func NewCOSS3Uploader(
	config *COSS3UploaderConfig,
) (Uploader, error) {

	conf := aws.NewConfig().
		WithEndpoint(config.ServiceEndpoint).
		WithCredentials(ibmiam.NewStaticCredentials(
			aws.NewConfig(),
			config.AuthEndpoint,
			config.ApiKey,
			config.ServiceInstanceID,
		)).WithS3ForcePathStyle(true)

	sess := session.Must(session.NewSession(conf))
	uploader := s3manager.NewUploader(sess)

	return &COSS3Uploader{
		COSS3UploaderConfig: *config,
		uploader:            uploader,
	}, nil
}

func (r *COSS3Uploader) UploadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	result, err := r.uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(r.COSS3UploaderConfig.Bucket),
		Key:         aws.String(filepath.Base(file.Name())),
		Body:        file,
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return errors.Wrap(err, "failed to upload file")
	}

	logger.Info("file uploaded", "url", result.Location)

	return nil
}

func provideCOSS3Config(
	ctx context.Context,
	cc ClientCommandRunner,
	deployedNamespace string,
	log logr.Logger,
) (*COSS3UploaderConfig, error) {
	secret := &corev1.Secret{}

	result, _ := cc.Do(ctx,
		GetAction(types.NamespacedName{
			Name:      utils.RHM_COS_UPLOADER_SECRET,
			Namespace: deployedNamespace,
		}, secret))
	if !result.Is(Continue) {
		return nil, result
	}

	configYamlBytes, ok := secret.Data["config.yaml"]
	if !ok {
		return nil, errors.New("rhm-cos-uploader-secret does not contain a config.yaml")
	}

	cosS3UploaderConfig := &COSS3UploaderConfig{}

	err := yaml.Unmarshal(configYamlBytes, &cosS3UploaderConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal rhm-cos-uploader-secret config.yaml")
	}

	return cosS3UploaderConfig, nil
}
