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

package uploaders

import (
	"bytes"
	"context"
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

	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
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

func (re *ReportJobError) Error() string {
	return re.ErrorMessage
}

func (re *ReportJobError) Unwrap() error { return re.Err }
