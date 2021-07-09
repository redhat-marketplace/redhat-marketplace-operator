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

	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/runtime"
)

type UploadTask struct {
	CC        ClientCommandRunner
	K8SClient rhmclient.SimpleClient
	Ctx       context.Context
	Config    *Config
	K8SScheme *runtime.Scheme
	Downloader
	Admin
}

func (r *UploadTask) Upload() error {
	logger.Info("upload task run start")

	// List the files from DataService
	fileList, err := r.Downloader.ListFiles()
	if err != nil {
		return err
	}

	for _, file := range fileList {
		// Download the file from DataService
		logger.Info("dac debug", "Download File", file)

		fileBytes, err := r.Downloader.DownloadFile(file)
		if err != nil {
			return err
		}
		logger.Info("dac debug", "Downloaded File", file)

		// Upload the file to Insights

		// Mark the file as deleted in DataService
		logger.Info("dac debug", "Deleting File", file)
		err = r.Admin.DeleteFile(file)
		if err != nil {
			return err
		}
		logger.Info("dac debug", "Deleted File", file)

		logger.Info("dac debug", "fileBytes", fileBytes)

	}

	return nil
}
