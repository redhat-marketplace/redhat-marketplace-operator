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
)

type EventReporter struct {
	log               logr.Logger
	dataServiceConfig *dataservice.DataServiceConfig
	dataService       *dataservice.DataService
}

func NewEventReporter(
	log logr.Logger,
	dataServiceConfig *dataservice.DataServiceConfig,
) (*EventReporter, error) {

	dataService, err := dataservice.NewDataService(dataServiceConfig)
	if err != nil {
		return nil, err
	}

	return &EventReporter{
		log:               log.WithValues("process", "EventReporter"),
		dataServiceConfig: dataServiceConfig,
		dataService:       dataService,
	}, nil
}

func (r *EventReporter) Report(metadata Metadata, eventJsons EventJsons) error {
	dir, err := os.MkdirTemp(r.dataServiceConfig.OutputPath, "datareporter-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	archiveFilePath, err := r.writeReport(dir, metadata, eventJsons)
	if err != nil {
		return err
	}

	if err := r.uploadReport(archiveFilePath); err != nil {
		return err
	}

	return nil
}

// write the report to disk and return archivePath
func (r *EventReporter) writeReport(dir string, metadata Metadata, eventJsons EventJsons) (string, error) {

	// subdir for json files
	filesDir := filepath.Join(dir, "archive")
	err := os.Mkdir(filesDir, 0700)
	if err != nil && !os.IsExist(err) {
		return "", err
	}

	// Generate and write manifest
	manifest := Manifest{Type: "dataReporter", Metadata: metadata}

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

	eventsFilePath := filepath.Join(filesDir, "events.json")
	if err = os.WriteFile(eventsFilePath, eventsBytes, 0600); err != nil {
		r.log.Error(err, "failed to write events file", "file", eventsFilePath)
		return "", err
	}

	// Create the archive
	archiveFilePath := filepath.Join(dir, fmt.Sprintf("data-reporter-%s.tar.gz", uuid.New()))
	f, err := os.Create(archiveFilePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := Tar(filesDir, f); err != nil {
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	r.dataService.UploadFile(ctx, archiveFilePath, archiveFile)
	return nil
}
