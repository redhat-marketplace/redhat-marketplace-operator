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
	"context"
	"io"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	logger = logf.Log.WithName("uploaders")
)

type UploaderTargets []UploaderTarget

type UploaderTarget interface {
	Name() string
}

var (
	UploaderTargetRedHatInsights UploaderTarget = &RedHatInsightsUploader{}
	UploaderTargetNoOp           UploaderTarget = &NoOpUploader{}
	UploaderTargetLocalPath      UploaderTarget = &LocalFilePathUploader{}
	UploaderTargetCOSS3          UploaderTarget = &COSS3Uploader{}
	UploaderTargetMarketplace    UploaderTarget = &MarketplaceUploader{}
	UploaderTargetDataService    UploaderTarget = &dataservice.DataService{}
)

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

func (u *MarketplaceUploader) Name() string {
	return "redhat-marketplace"
}

func MustParseUploaderTarget(s string) UploaderTarget {
	switch s {
	//  Disable redhat-insights uploader option. Motivation: requires cluster secret read.
	//	case UploaderTargetRedHatInsights.Name():
	//		return UploaderTargetRedHatInsights
	case UploaderTargetLocalPath.Name():
		return UploaderTargetLocalPath
	case UploaderTargetNoOp.Name():
		return UploaderTargetNoOp
	case UploaderTargetCOSS3.Name():
		return UploaderTargetCOSS3
	case UploaderTargetMarketplace.Name():
		return UploaderTargetMarketplace
	case UploaderTargetDataService.Name():
		return UploaderTargetDataService
	default:
		panic(errors.Errorf("provided string is not a valid upload target %s", s))
	}
}

type Uploaders []Uploader

type Uploader interface {
	UploaderTarget
	UploadFile(ctx context.Context, fileName string, reader io.Reader) (id string, err error)
}

type NoOpUploader struct{}

var _ Uploader = &NoOpUploader{}

func (r *NoOpUploader) UploadFile(ctx context.Context, fileName string, reader io.Reader) (string, error) {
	log := logger.WithValues("uploader", "noop")
	log.Info("upload is a no op")
	return "", nil
}

type ReportJobError struct {
	ErrorMessage string
	Err          error
}

func (re *ReportJobError) Error() string {
	return re.ErrorMessage
}

func (re *ReportJobError) Unwrap() error { return re.Err }
