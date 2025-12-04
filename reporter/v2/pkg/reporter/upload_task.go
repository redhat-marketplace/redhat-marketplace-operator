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
	"fmt"
	"os"
	"strconv"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	dataservicev1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/marketplace/v1alpha1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UploadRun interface {
	RunGeneric(ctx context.Context) error
	RunReport(ctx context.Context, report *marketplacev1alpha1.MeterReport) error
}

// Task to upload the reports from dataservice to redhat-insights
type UploadTask struct {
	logger      logr.Logger
	config      *Config
	k8SClient   rhmclient.SimpleClient
	k8SScheme   *runtime.Scheme
	fileStorage dataservice.FileStorage
	uploaders   uploaders.Uploaders
}

// RunReport uses status fields on the Report to get identifiers
// for the file
func (r *UploadTask) RunReport(ctx context.Context, report *marketplacev1alpha1.MeterReport) error {
	logger := r.logger
	logger.Info("upload task run report", "name", report.Name)

	if report.Status.DataServiceStatus == nil || report.Status.DataServiceStatus.ID == "" {
		return errors.New("Report does not have an DataService file ID")
	}

	fileID := report.Status.DataServiceStatus.ID

	file, err := r.fileStorage.GetFile(ctx, fileID)
	if err != nil {
		logger.Error(err, "error getting file")
		return err
	}

	statuses := r.uploadFile(ctx, file)
	success, condition := findStatus(statuses)

	if err = updateMeterReportStatus(ctx, r.k8SClient, report.Name, report.Namespace,
		func(m marketplacev1alpha1.MeterReportStatus) marketplacev1alpha1.MeterReportStatus {
			m.Conditions.SetCondition(condition)
			m.UploadStatus.Append(statuses)

			if !success {
				m.RetryUpload = m.RetryUpload + 1
			}

			dataServiceStatus := m.UploadStatus.Get(uploaders.UploaderTargetDataService.Name())

			if dataServiceStatus != nil {
				m.DataServiceStatus = dataServiceStatus
			}

			return m
		}); err != nil {
		return err
	}

	if success {
		if err = r.deleteFile(ctx, file); err != nil {
			logger.Error(err, "failed to delete file from dataservice")
		}
	} else {
		logger.Info("Upload to backend failed, skipping file deletion from dataservice")
	}

	return nil
}

const uploadAttempts = "uploadAttempts"

// Run checks for just files in DataService and sends them if it can.
func (r *UploadTask) RunGeneric(ctx context.Context) error {
	logger := r.logger
	logger.Info("upload task run generic start")

	retry := true
	for retry {

		// List the files from DataService
		logger.Info("Listing files in data-service")
		fileList, err := r.fileStorage.ListFiles(ctx)

		if len(fileList) != 0 && err != nil { // Partial list returned, but likely too many to list, upload and retry
			logger.Error(err, "partial ListFiles returned, proceeding.")
		} else if err != nil {
			return err
		} else { // Full list returned, upload and quit
			retry = false
		}

		logger.Info("ListFiles", "files", fileList)

		for _, file := range fileList {
			if file.GetDeletedAt() != nil && !file.GetDeletedAt().AsTime().IsZero() {
				logger.Info("Skipping deleted file")
				continue
			}

			fileId := file.Id
			file, err = r.fileStorage.GetFile(ctx, file.Id)
			if err != nil {
				logger.Error(err, "failed to get file", "id", fileId)
				continue
			}

			if file.Metadata == nil {
				file.Metadata = map[string]string{}
			}

			var uploadAttemptsInt int
			if uploadAttemptsStr, ok := file.Metadata[uploadAttempts]; ok {
				uploadAttemptsInt, err = strconv.Atoi(uploadAttemptsStr)
				if err != nil {
					uploadAttemptsInt = 0
				}
			}

			if uploadAttemptsInt >= maxUploadAttempts {
				continue
			}

			statuses := r.uploadFile(ctx, file)
			success, _ := findStatus(statuses)

			if !success && uploadAttemptsInt < maxUploadAttempts {
				logger.Info("failed to complete upload without an issue, will not delete the file", "attempts", uploadAttemptsInt)
				uploadAttemptsInt = uploadAttemptsInt + 1
				file.Metadata[uploadAttempts] = fmt.Sprintf("%d", uploadAttemptsInt)
				err := r.fileStorage.UpdateMetadata(ctx, file)
				if err != nil {
					logger.Error(err, "failed to update metadata")
				}
				continue
			}

			if err = r.deleteFile(ctx, file); err != nil {
				logger.Error(err, "failed to delete metadata")
			}
		}
	}

	return nil
}

func (r *UploadTask) uploadFile(
	ctx context.Context,
	file *dataservicev1.FileInfo,
) (statuses []*marketplacev1alpha1.UploadDetails) {
	logger := r.logger
	logger.Info("DownloadFile", "Downloading file from data-service", file)
	localFileName, downloadErr := r.fileStorage.DownloadFile(ctx, file)

	// Upload the file to uploaders
	for _, uploader := range r.uploaders {
		details := &marketplacev1alpha1.UploadDetails{}
		details.Target = uploader.Name()

		if downloadErr != nil {
			logger.Error(downloadErr, "failed to download file", "id", file.Id)
			details.Error = fmt.Sprintf("error: %s details: %+v", downloadErr.Error(), errors.GetDetails(downloadErr))
			details.Status = marketplacev1alpha1.UploadStatusFailure
			statuses = append(statuses, details)
			return
		}

		logger.Info("Uploaded file", "file", file, "target", uploader.Name())
		data, err := os.ReadFile(localFileName)

		onError := func(err error, msg string) {
			logger.Error(err, msg)
			details.Error = fmt.Sprintf("error: %s details: %+v", err.Error(), errors.GetDetails(err))
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
			logger.Error(err, "failed to upload file", errors.GetDetails(err)...)
			onError(err, "failed to upload file")
			continue
		}

		details.ID = id
		details.Status = marketplacev1alpha1.UploadStatusSuccess
		statuses = append(statuses, details)
	}

	return
}

func (r *UploadTask) deleteFile(ctx context.Context, file *dataservicev1.FileInfo) error {
	// Mark the file as deleted in DataService
	err := r.fileStorage.DeleteFile(ctx, file)
	if err != nil {
		logger.Error(err, "failed to delete a file")
		return err
	}
	logger.Info("DeleteFile", "Deleted file from data-service", file)
	return nil
}

func findStatus(statuses marketplacev1alpha1.UploadDetailConditions) (success bool, condition status.Condition) {
	condition = marketplacev1alpha1.ReportConditionUploadStatusUnknown
	success = false

	// if one of our required uploaders work it's a success
	if statuses.OneSuccessOf(
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
