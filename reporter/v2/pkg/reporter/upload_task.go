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

package reporter

import (
	"bytes"
	"context"
	"os"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Task to upload the reports from dataservice to redhat-insights
type UploadTask struct {
	logger      logr.Logger
	config      *Config
	k8SClient   rhmclient.SimpleClient
	k8SScheme   *runtime.Scheme
	fileStorage dataservice.FileStorage
	uploaders   uploaders.Uploaders
}

func (r *UploadTask) Run(ctx context.Context) error {
	logger := r.logger
	logger.Info("upload task run start")

	// List the files from DataService
	logger.Info("Listing files in data-service")
	fileList, err := r.fileStorage.ListFiles(ctx)
	if err != nil {
		return err
	}
	logger.Info("ListFiles", "Listed files in data-service", fileList)

	for _, file := range fileList {
		if file.GetDeletedAt() != nil && !file.GetDeletedAt().AsTime().IsZero() {
			logger.Info("Skipping deleted file")
			continue
		}

		// Download the file from DataService
		logger.Info("DownloadFile", "Downloading file from data-service", file)
		localFileName, err := r.fileStorage.DownloadFile(ctx, file)
		if err != nil {
			logger.Error(err, "failed to download file", "id", file.Id)
			continue
		}

		logger.Info("DownloadFile", "Downloaded file from data-service", file)
		statuses := []*marketplacev1alpha1.UploadDetails{}

		// Upload the file to uploaders
		for _, uploader := range r.uploaders {
			details := &marketplacev1alpha1.UploadDetails{}
			details.Target = uploader.Name()

			logger.Info("Uploaded file", "file", file, "target", uploader.Name())
			data, err := os.ReadFile(localFileName)

			onError := func(err error, msg string) {
				logger.Error(err, msg)
				details.Error = err.Error()
				details.Status = marketplacev1alpha1.UploadStatusFailure
				statuses = append(statuses, details)
			}

			if err != nil {
				onError(err, "failed to open local file")
				continue
			}

			var id string
			id, err = uploader.UploadFile(ctx, file.Name, bytes.NewReader(data))

			if err != nil {
				logger.Error(err, "failed to upload a file")
				onError(err, "failed to open local file")
				continue
			}

			details.ID = id
			details.Status = marketplacev1alpha1.UploadStatusSuccess
			statuses = append(statuses, details)
		}

		metadata := &dataservice.MeterReportMetadata{}
		if err := metadata.From(file.Metadata); err != nil {
			logger.Error(err, "error parsing metadata")
			continue
		}

		if metadata.IsEmpty() {
			continue
		}

		success, condition := findStatus(statuses)

		if err := updateMeterReportStatus(ctx, r.k8SClient, metadata.ReportName, metadata.ReportNamespace,
			func(m marketplacev1alpha1.MeterReportStatus) marketplacev1alpha1.MeterReportStatus {
				m.Conditions.SetCondition(condition)
				m.UploadStatus.Append(statuses)

				dataServiceStatus := m.UploadStatus.Get(uploaders.UploaderTargetDataService.Name())

				if dataServiceStatus != nil {
					m.DataServiceStatus = dataServiceStatus
				}

				return m
			}); err != nil {
			logger.Error(err, "failed to update meter report")
			continue
		}

		if !success {
			logger.Info("failed to complete upload without an issue, will not delete the file")
			continue
		}

		// Mark the file as deleted in DataService
		err = r.fileStorage.DeleteFile(ctx, file)
		if err != nil {
			logger.Error(err, "failed to delete a file")
			continue
		}
		logger.Info("DeleteFile", "Deleted file from data-service", file)
	}

	return nil
}

func findStatus(statuses marketplacev1alpha1.UploadDetailConditions) (success bool, condition status.Condition) {
	condition = marketplacev1alpha1.ReportConditionUploadStatusUnknown
	success = false

	// if one of our required uploaders work it's a success
	if statuses.OneSucessOf(
		[]string{
			uploaders.UploaderTargetMarketplace.Name(),
			uploaders.UploaderTargetRedHatInsights.Name(),
		},
	) {
		success = true
		condition = marketplacev1alpha1.ReportConditionUploadStatusFinished
		return
	}

	// none of our required uploaders worked - it's not a success
	if err := statuses.Errors(); err != nil {
		condition = marketplacev1alpha1.ReportConditionUploadStatusErrored
		condition.Message = err.Error()
		return
	}

	return
}

func updateMeterReportStatus(
	ctx context.Context,
	k8sClient client.Client,
	name, namespace string,
	mutate func(marketplacev1alpha1.MeterReportStatus) marketplacev1alpha1.MeterReportStatus,
) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		meterReport := marketplacev1alpha1.MeterReport{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &meterReport)

		if err != nil {
			return err
		}

		meterReport.Status = mutate(meterReport.Status)

		if err != nil {
			return err
		}

		return k8sClient.Status().Update(ctx, &meterReport)
	})
}
