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
	"context"
	"io"
	"path/filepath"

	"emperror.dev/errors"
	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3/s3manager"
)

var _ Uploader = &COSS3Uploader{}

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

func (r *COSS3Uploader) UploadFile(ctx context.Context, path string, file io.Reader) (string, error) {
	name := filepath.Base(path)

	result, err := r.uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(r.COSS3UploaderConfig.Bucket),
		Key:         aws.String(name),
		Body:        file,
		ContentType: aws.String("application/gzip"),
	})

	if err != nil {
		return "", errors.Wrap(err, "failed to upload file")
	}

	logger.Info("file uploaded", "url", result.Location)
	return result.UploadID, nil
}
