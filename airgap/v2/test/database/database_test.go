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

package database_test

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var logger logr.Logger
var dbName = "test.db"

func initLog() error {
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	logger = zapr.NewLogger(zapLog)
	return nil
}

func closeDBConnection(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Error: Couldn't close Database: %v", err)
	}

	sqlDB.Close()
	os.Remove(dbName)
}

func TestDatabase_SaveFile(t *testing.T) {
	err := initLog()
	if err != nil {
		t.Fatalf("Couldn't initialize logger: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("Couldn't create sqlite connection")
	}
	defer closeDBConnection(db)

	//Perform migrations
	db.AutoMigrate(&models.FileMetadata{}, &models.File{}, &models.Metadata{})

	bs := make([]byte, 1024)
	finfo := &v1.FileInfo{
		FileId: &v1.FileID{
			Data: &v1.FileID_Name{
				Name: "test-file",
			},
		},
		Size:            1024,
		Compression:     true,
		CompressionType: "gzip",
		Metadata: map[string]string{
			"Key1": "Value1",
			"Key2": "Value2",
		},
	}

	database := &database.Database{
		DB:  db,
		Log: logger,
	}

	tests := []struct {
		name   string
		finfo  *v1.FileInfo
		bs     []byte
		m      *models.Metadata
		errMsg string
	}{
		{
			name:  "save file to database",
			finfo: finfo,
			bs:    bs,
			m: &models.Metadata{
				Size:         1024,
				ProvidedName: "test-file",
				FileMetadata: []models.FileMetadata{
					{Key: "Key1", Value: "Value1"},
					{Key: "Key2", Value: "Value2"},
				},
			},
		},
		{
			name:   "invalid method call to save file with nil file info",
			finfo:  nil,
			bs:     bs,
			m:      nil,
			errMsg: fmt.Sprintf("nil arguments received: finfo: %v bs: %v", nil, bs),
		},
		{
			name:   "invalid method call to save file with nil byte slice",
			finfo:  finfo,
			bs:     nil,
			m:      nil,
			errMsg: fmt.Sprintf("nil arguments received: finfo: %v bs: %v", finfo, make([]byte, 0)),
		},
		{
			name: "invalid method call to save file with whitespaces in the name",
			finfo: &v1.FileInfo{
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: "    ",
					},
				},
			},
			bs:     bs,
			m:      nil,
			errMsg: "file id/name is blank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := database.SaveFile(tt.finfo, tt.bs)
			if err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message: %v, instead got: %v", tt.errMsg, err.Error())
				}
			} else if len(tt.errMsg) > 0 {
				t.Errorf("Expected error: %v was never received!", tt.errMsg)
			}

			if tt.m != nil {
				m := &models.Metadata{}
				db.Preload(clause.Associations).Order("created_at desc").First(&m)

				if tt.m.ProvidedName != m.ProvidedName {
					t.Errorf("Expected file name: %v, instead got: %v", tt.m.ProvidedName, m.ProvidedName)
				}

				if tt.m.Size != m.Size {
					t.Errorf("Expected file size: %v, instead got: %v", tt.m.Size, m.Size)
				}

				if len(tt.m.FileMetadata) != len(m.FileMetadata) {
					t.Errorf("Expected metadata keys: %v, instead got: %v", tt.m.Size, m.Size)
				}
			}
		})
	}
}

func TestDatabase_DownloadFile(t *testing.T) {
	err := initLog()
	if err != nil {
		t.Fatalf("Couldn't initialize logger: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("Couldn't create sqlite connection")
	}
	defer closeDBConnection(db)

	//Perform migrations
	db.AutoMigrate(&models.FileMetadata{}, &models.File{}, &models.Metadata{})

	bs := make([]byte, 1024)
	finfo := &v1.FileInfo{
		FileId: &v1.FileID{
			Data: &v1.FileID_Name{
				Name: "test-file",
			},
		},
		Size:            1024,
		Compression:     true,
		CompressionType: "gzip",
		Metadata: map[string]string{
			"Key1": "Value1",
			"Key2": "Value2",
		},
	}

	database := &database.Database{
		DB:  db,
		Log: logger,
	}

	//Save a file in database to retreive in tests later
	err = database.SaveFile(finfo, bs)
	if err != nil {
		t.Fatalf("Failed to create seed data for tests due to error: %v", err)
	}

	tests := []struct {
		name   string
		fid    *v1.FileID
		m      *models.Metadata
		errMsg string
	}{
		{
			name: "download a file that exists in the database",
			fid: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "test-file",
				},
			},
			m: &models.Metadata{
				Size:         1024,
				ProvidedName: "test-file",
				FileMetadata: []models.FileMetadata{
					{Key: "Key1", Value: "Value1"},
					{Key: "Key2", Value: "Value2"},
				},
			},
			errMsg: "",
		},
		{
			name: "invalid method call to download a file that doesn't exist",
			fid: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "dontexist.zip",
				},
			},
			m:      nil,
			errMsg: fmt.Sprintf("no file found for provided_name: %v / provided_id: %v", "dontexist.zip", ""),
		},
		{
			name: "invalid method call to download a file with whitespaces as the name",
			fid: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "   ",
				},
			},
			m:      nil,
			errMsg: "file id/name is blank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := database.DownloadFile(tt.fid)
			if err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message: %v, instead got: %v", tt.errMsg, err.Error())
				}
			} else if len(tt.errMsg) > 0 {
				t.Errorf("Expected error: %v was never received!", tt.errMsg)
			}

			if tt.m != nil {
				if tt.m.ProvidedName != m.ProvidedName {
					t.Errorf("Expected file name: %v, instead got: %v", tt.m.ProvidedName, m.ProvidedName)
				}

				if tt.m.Size != m.Size {
					t.Errorf("Expected file size: %v, instead got: %v", tt.m.Size, m.Size)
				}

				if len(tt.m.FileMetadata) != len(m.FileMetadata) {
					t.Errorf("Expected metadata keys: %v, instead got: %v", tt.m.Size, m.Size)
				}
			}
		})
	}
}

func TestDatabase_ListFileMetadata(t *testing.T) {
	err := initLog()
	if err != nil {
		t.Fatalf("Couldn't initialize logger: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("Couldn't create sqlite connection")
	}
	defer closeDBConnection(db)

	//Perform migrations
	db.AutoMigrate(&models.FileMetadata{}, &models.File{}, &models.Metadata{})
	db_ := &database.Database{
		DB:  db,
		Log: logger,
	}

	current_time := time.Now().Unix()
	populateDataset(db_)

	tests := []struct {
		name          string
		conditionList []*database.Condition
		sortList      []*database.SortOrder
		m             []*models.Metadata
		errMsg        string
	}{
		{
			name: "fetch file metadata based on provided name",
			conditionList: []*database.Condition{
				{
					Key:      "provided_name",
					Operator: "LIKE",
					Value:    "marketplace_report.zip",
				},
			},
			sortList: []*database.SortOrder{
				{
					Key:   "size",
					Order: "ASC",
				},
			},
			m: []*models.Metadata{
				{
					ProvidedName: "marketplace_report.zip",
					Size:         200,
					FileMetadata: []models.FileMetadata{
						{Key: "type", Value: "report"},
						{Key: "version", Value: "2"},
					},
				},
			},
			errMsg: "",
		},
		{
			name: "fetch file list based on file_metadata only ",
			conditionList: []*database.Condition{
				{
					Key:      "description",
					Operator: "LIKE",
					Value:    "DOS",
				},
			},
			sortList: []*database.SortOrder{},
			m: []*models.Metadata{
				{
					ProvidedName: "dosbox",
					Size:         1500,
					FileMetadata: []models.FileMetadata{
						{
							Key:   "version",
							Value: "4.3",
						},
						{
							Key:   "description",
							Value: "Emulator with builtin DOS for running DOS Games",
						},
					},
				},
				{
					ProvidedName: "dosfstools",
					Size:         2000,
					FileMetadata: []models.FileMetadata{
						{
							Key:   "version",
							Value: "latest",
						},
						{
							Key:   "description",
							Value: "DOS filesystem utilities",
						},
					},
				},
			},
			errMsg: "",
		},
		{
			name: "fetch file list based on file name and file_metadata",
			conditionList: []*database.Condition{
				{
					Key:      "description",
					Operator: "LIKE",
					Value:    "DOS",
				},
				{
					Key:      "provided_name",
					Operator: "LIKE",
					Value:    "dos",
				},
				{
					Key:      "version",
					Operator: "LIKE",
					Value:    "4.3",
				},
			},
			sortList: []*database.SortOrder{},
			m: []*models.Metadata{
				{
					ProvidedName: "dosbox",
					Size:         1500,
					FileMetadata: []models.FileMetadata{
						{
							Key:   "version",
							Value: "4.3",
						},
						{
							Key:   "description",
							Value: "Emulator with builtin DOS for running DOS Games",
						},
					},
				},
			},
			errMsg: "",
		},
		{
			name: "fetch nonexisting file",
			conditionList: []*database.Condition{
				{
					Key:      "provided_name",
					Operator: "=",
					Value:    "nonexisting.zip",
				},
			},
			sortList: []*database.SortOrder{},
			m:        []*models.Metadata{},
			errMsg:   "",
		},
		{ // this test might fail if more than one save file opeation take too long to execute
			name: "fetch list of file created between provided time range",
			conditionList: []*database.Condition{
				{
					Key:      "created_at",
					Operator: ">",
					Value:    strconv.FormatInt(current_time+2, 10),
				},
				{
					Key:      "created_at",
					Operator: "<",
					Value:    strconv.FormatInt(current_time+6, 10),
				},
			},
			sortList: []*database.SortOrder{
				{
					Key:   "size",
					Order: "ASC",
				},
			},
			m: []*models.Metadata{
				{
					ProvidedName: "marketplace_report.zip",
					Size:         200,
					FileMetadata: []models.FileMetadata{
						{
							Key:   "version",
							Value: "2",
						},
						{
							Key:   "type",
							Value: "marketplace_report",
						},
					},
				},
				{
					ProvidedName: "airgap-deploy.zip",
					Size:         1000,
					FileMetadata: []models.FileMetadata{
						{
							Key:   "version",
							Value: "latest",
						},
						{
							Key:   "name",
							Value: "airgap",
						},
						{
							Key:   "type",
							Value: "deployment-package",
						},
						{
							Key:   "description",
							Value: "airgap deployment code",
						},
					},
				},
			},
			errMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := db_.ListFileMetadata(tt.conditionList, tt.sortList)
			if err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message: %v, instead got: %v", tt.errMsg, err.Error())
				}
			} else if len(tt.errMsg) > 0 {
				t.Errorf("Expected error: %v was never received!", tt.errMsg)
			}

			if len(tt.m) != len(m) {
				t.Errorf("Expected length of filelist: %v, instead got: %v", len(tt.m), len(m))
			}
			for i := range m {
				if m[i].ProvidedName != tt.m[i].ProvidedName {
					t.Errorf("Expected file name: %v, instead got: %v", tt.m[i].ProvidedName, m[i].ProvidedName)
				}
				if tt.m[i].Size != m[i].Size {
					t.Errorf("Expected file size: %v, instead got: %v", tt.m[i].Size, m[i].Size)
				}

				if len(tt.m[i].FileMetadata) != len(m[i].FileMetadata) {
					t.Errorf("Expected metadata keys: %v, instead got: %v", tt.m[i].FileMetadata, m[i].FileMetadata)
				}
			}
		})
	}
}

// Populate Database for testing
func populateDataset(database *database.Database) {
	var t *testing.T
	files := []v1.FileInfo{
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "reports.zip",
				},
			},
			Size:            1000,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version": "1",
				"type":    "report",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "reports.zip",
				},
			},
			Size:            2000,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version": "2",
				"type":    "report",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "marketplace_report.zip",
				},
			},
			Size:            300,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version": "1",
				"type":    "marketplace_report",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "marketplace_report.zip",
				},
			},
			Size:            200,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version": "2",
				"type":    "marketplace_report",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "airgap-deploy.zip",
				},
			},
			Size:            1000,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version": "1",
				"name":    "airgap",
				"type":    "deployment-package",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "airgap-deploy.zip",
				},
			},
			Size:            1000,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version":     "latest",
				"name":        "airgap",
				"type":        "deployment-package",
				"description": "airgap deployment code ",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "Kube.sh",
				},
			},
			Size:            200,
			Compression:     false,
			CompressionType: "",
			Metadata: map[string]string{
				"version":     "latest",
				"description": "kube cluster executable file",
				"type":        "kube-executable",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "dosfstools",
				},
			},
			Size:            2000,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version":     "latest",
				"description": "DOS filesystem utilities",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "dosbox",
				},
			},
			Size:            1500,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version":     "4.3",
				"description": "Emulator with builtin DOS for running DOS Games",
			},
		},
	}

	for i := range files {
		bs := make([]byte, files[i].Size)
		dbErr := database.SaveFile(&files[i], bs)
		if dbErr != nil {
			t.Fatalf("Couldn't save file due to:%v", dbErr)
		}
		time.Sleep(1 * time.Second)
	}
}
