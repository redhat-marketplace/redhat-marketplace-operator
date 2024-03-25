// Copyright 2023 IBM Corp.
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

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
)

type EventReporter struct {
	log         logr.Logger
	config      *Config
	tarGzipPool TarGzipPool
	dataService *dataservice.DataService
}

func NewEventReporter(
	log logr.Logger,
	config *Config,
	dataService *dataservice.DataService,
) (*EventReporter, error) {

	return &EventReporter{
		log:         log.WithValues("process", "EventReporter"),
		config:      config,
		tarGzipPool: TarGzipPool{},
		dataService: dataService,
	}, nil
}

func (r *EventReporter) Report(metadata Metadata, manifestType string, eventJsons EventJsons) error {

	dir, err := os.MkdirTemp(r.config.OutputDirectory, "datareporter-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	archiveFilePath, err := r.writeReport(dir, metadata, manifestType, eventJsons)
	if err != nil {
		return err
	}

	if err := r.uploadReport(archiveFilePath); err != nil {
		return err
	}

	return nil
}

// write the report to disk and return archivePath
func (r *EventReporter) writeReport(dir string, metadata Metadata, manifestType string, eventJsons EventJsons) (string, error) {

	// subdir for json files
	filesDir := filepath.Join(dir, "archive")
	err := os.Mkdir(filesDir, 0700)
	if err != nil && !os.IsExist(err) {
		return "", err
	}

	// Generate and write manifest
	manifest := Manifest{Type: manifestType, Version: "1"}

	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}

	manifestFilePath := filepath.Join(filesDir, "manifest.json")
	if err = os.WriteFile(manifestFilePath, manifestBytes, 0600); err != nil {
		r.log.Error(err, "failed to write manifest file", "file", manifestFilePath)
		return "", err
	}

	// Generate and write events
	reportData := ReportData{Metadata: metadata, EventJsons: eventJsons}
	eventsBytes, err := json.Marshal(reportData)
	if err != nil {
		return "", err
	}

	fileid := uuid.New()

	eventsFilePath := filepath.Join(filesDir, fmt.Sprintf("%s.json", fileid))
	if err = os.WriteFile(eventsFilePath, eventsBytes, 0600); err != nil {
		r.log.Error(err, "failed to write events file", "file", eventsFilePath)
		return "", err
	}

	// Create the archive
	archiveFilePath := filepath.Join(dir, fmt.Sprintf("data-reporter-%s.tar.gz", fileid))

	if err = r.tarGzipPool.TarGzip(filesDir, archiveFilePath); err != nil {
		return "", err
	}

	return archiveFilePath, nil
}

// upload the report to data-service
func (r *EventReporter) uploadReport(archiveFilePath string) error {
	archiveFile, err := os.Open(archiveFilePath)
	if err != nil {
		return err
	}
	defer archiveFile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// The token rotates
	// Set these opts on a per call basis (NewStream), instead of per connection
	opts, err := dataservice.ProvideGRPCCallOptions(r.config.DataServiceTokenFile)
	if err != nil {
		return err
	}

	r.dataService.SetCallOpts(opts...)

	id, err := r.dataService.UploadFile(ctx, archiveFilePath, archiveFile)
	if err != nil {
		return err
	}
	r.log.Info("Uploaded file", "id", id)

	return nil
}

func (r *EventReporter) provideDataServiceConfig() (*dataservice.DataServiceConfig, error) {
	cert, err := os.ReadFile(r.config.DataServiceCertFile)
	if err != nil {
		return nil, err
	}

	var serviceAccountToken = ""
	if r.config.DataServiceTokenFile != "" {
		content, err := os.ReadFile(r.config.DataServiceTokenFile)
		if err != nil {
			return nil, err
		}
		serviceAccountToken = string(content)
	}

	var dataServiceDNS = fmt.Sprintf("%s.%s.svc:8004", utils.DATA_SERVICE_NAME, r.config.Namespace)

	return &dataservice.DataServiceConfig{
		Address:          dataServiceDNS,
		DataServiceToken: serviceAccountToken,
		DataServiceCert:  cert,
		OutputPath:       r.config.OutputDirectory,
	}, nil
}

func provideDataServiceConfig(
	dataServiceCertFile string,
	dataServiceTokenFile string,
	namespace string,
	outputDir string,
) (*dataservice.DataServiceConfig, error) {

	cert, err := os.ReadFile(dataServiceCertFile)
	if err != nil {
		return nil, err
	}

	var serviceAccountToken = ""
	if dataServiceTokenFile != "" {
		content, err := os.ReadFile(dataServiceTokenFile)
		if err != nil {
			return nil, err
		}
		serviceAccountToken = string(content)
	}

	var dataServiceDNS = fmt.Sprintf("%s.%s.svc:8004", utils.DATA_SERVICE_NAME, namespace)

	return &dataservice.DataServiceConfig{
		Address:          dataServiceDNS,
		DataServiceToken: serviceAccountToken,
		DataServiceCert:  cert,
		OutputPath:       outputDir,
	}, nil
}
