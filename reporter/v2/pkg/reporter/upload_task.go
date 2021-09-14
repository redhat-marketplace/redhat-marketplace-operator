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
	"context"

	"emperror.dev/errors"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
)

// Task to upload the reports from dataservice to redhat-insights

type UploadTask struct {
	K8SClient rhmclient.SimpleClient
	Ctx       context.Context
	Config    *Config
	K8SScheme *runtime.Scheme
	Downloader
	Uploaders
	Admin
}

func (r *UploadTask) Run() error {
	logger.Info("upload task run start")

	// List the files from DataService
	logger.Info("Listing files in data-service")
	fileList, err := r.Downloader.ListFiles()
	if err != nil {
		return err
	}
	logger.Info("ListFiles", "Listed files in data-service", fileList)

	for _, file := range fileList {
		// Download the file from DataService
		logger.Info("DownloadFile", "Downloading file from data-service", file)
		localFileName, err := r.Downloader.DownloadFile(file)
		if err != nil {
			return err
		}
		logger.Info("DownloadFile", "Downloaded file from data-service", file)

		// Upload the file to Insights
		for _, uploader := range r.Uploaders {
			logger.Info("Uploaded file to redhat-insights", "file", file, "target", uploader.Name())
			err = uploader.UploadFile(localFileName)
			if err != nil {
				return errors.WithDetails(err, "error uploading file", "target", uploader.Name())
			}
		}

		// Mark the file as deleted in DataService
		logger.Info("DeleteFile", "Deleting file from data-service", file)
		err = r.Admin.DeleteFile(file)
		if err != nil {
			return err
		}
		logger.Info("DeleteFile", "Deleted file from data-service", file)
	}

	return nil
}
