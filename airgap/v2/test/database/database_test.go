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
	"bytes"
	"fmt"
	"log"
	"os"
	"reflect"
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
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var logger logr.Logger
var dbName = "test.db"
var before = 1577836800 // Epoch time which equates to 1st Jan 2020
var after = 1577923200  // Epoch time which equates to 2nd Jan 2020

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

	database := &database.Database{
		DB:  db,
		Log: logger,
	}
	populateDataset(database, t)

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
					Name: "reports.zip",
				},
			},
			m: &models.Metadata{
				Size:         2000,
				ProvidedName: "reports.zip",
				FileMetadata: []models.FileMetadata{
					{Key: "version", Value: "2"},
					{Key: "type", Value: "report"},
				},
				File: models.File{Content: []byte("reports.zip")},
			},
			errMsg: "",
		},
		{
			name: "download a file with purged content",
			fid: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "delete1.txt",
				},
			},
			m:      nil,
			errMsg: fmt.Sprintf("no file found for provided_name: %v / provided_id: %v", "delete1.txt", ""),
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

				if string(tt.m.File.Content) != string(bytes.Trim(m.File.Content, "\x00")) {
					t.Errorf("Expected file content %v, instead got: %v", string(tt.m.File.Content), string(m.File.Content))
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

	populateDataset(db_, t)

	tests := []struct {
		name                string
		conditionList       []*database.Condition
		sortList            []*database.SortOrder
		includeDeletedFiles bool
		m                   []*models.Metadata
		errMsg              string
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
			sortList:            []*database.SortOrder{},
			includeDeletedFiles: false,
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
			sortList:            []*database.SortOrder{},
			includeDeletedFiles: false,
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
			sortList:            []*database.SortOrder{},
			includeDeletedFiles: false,
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
		{
			name: "fetch list of file created between provided time range",
			conditionList: []*database.Condition{
				{
					Key:      "created_at",
					Operator: ">",
					Value:    strconv.FormatInt((int64(before) - 1), 10),
				},
				{
					Key:      "created_at",
					Operator: "<",
					Value:    strconv.FormatInt((int64(after) + 1), 10),
				},
			},
			sortList: []*database.SortOrder{
				{
					Key:   "size",
					Order: "ASC",
				},
			},
			includeDeletedFiles: false,
			m: []*models.Metadata{

				{
					ProvidedName: "marketplace_report.zip",
					Size:         200,
					FileMetadata: []models.FileMetadata{
						{
							Key:   "version",
							Value: "1",
						},
						{
							Key:   "type",
							Value: "marketplace_report",
						},
					},
				},
				{
					ProvidedName: "reports.zip",
					Size:         2000,
					FileMetadata: []models.FileMetadata{
						{
							Key:   "version",
							Value: "1",
						},
						{
							Key:   "type",
							Value: "report",
						},
					},
				},
			},
			errMsg: "",
		},
		{
			name: "fetch list of file marked for deletion",
			conditionList: []*database.Condition{
				{
					Key:      "provided_name",
					Operator: "=",
					Value:    "delete.txt",
				},
			},
			sortList:            []*database.SortOrder{},
			includeDeletedFiles: true,
			m: []*models.Metadata{
				{
					ProvidedName: "delete.txt",
					Size:         1500,
					FileMetadata: []models.FileMetadata{
						{
							Key:   "description",
							Value: "file marked for deletion",
						},
					},
				},
			},
			errMsg: "",
		},
		{
			name: "fetch list of deleted file",
			conditionList: []*database.Condition{
				{
					Key:      "provided_name",
					Operator: "=",
					Value:    "delete1.txt",
				},
			},
			sortList:            []*database.SortOrder{},
			includeDeletedFiles: true,
			m:                   []*models.Metadata{},
			errMsg:              "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := db_.ListFileMetadata(tt.conditionList, tt.sortList, tt.includeDeletedFiles)
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

func TestDatabase_TombstoneFile(t *testing.T) {
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
			name: "mark file for deletion on download",
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
			err := database.TombstoneFile(tt.fid)
			if err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message: %v, instead got: %v", tt.errMsg, err.Error())
				}
			} else if len(tt.errMsg) > 0 {
				t.Errorf("Expected error: %v was never received!", tt.errMsg)
			}
			if tt.m != nil {
				m := &models.Metadata{}
				db.Where("provided_name = ?", tt.m.ProvidedName).Order("created_at desc").Find(m)
				t.Log(m)
				if m.CleanTombstoneSetAt == 0 {
					t.Errorf("Expected file is not marked for deletion, Expected tombestone: %v , got %v for file %v ", tt.m.CleanTombstoneSetAt, m.CleanTombstoneSetAt, tt.m.ProvidedName)
				}
			}
		})
	}
}

func TestDatabase_GetFileMetadata(t *testing.T) {
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

	database := &database.Database{
		DB:  db,
		Log: logger,
	}

	populateDataset(database, t)

	tests := []struct {
		name   string
		fid    *v1.FileID
		m      *models.Metadata
		errMsg string
	}{
		{
			name: "Get metadata for a file that exists in the database",
			fid: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "reports.zip",
				},
			},
			m: &models.Metadata{
				Size:         2000,
				ProvidedName: "reports.zip",
				FileMetadata: []models.FileMetadata{
					{Key: "version", Value: "2"},
					{Key: "type", Value: "report"},
				},
			},
			errMsg: "",
		},
		{
			name: "invalid request to get metadata of a file that doesn't exist",
			fid: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "dontexist.zip",
				},
			},
			m:      nil,
			errMsg: fmt.Sprintf("no file found for provided_name: %v / provided_id: %v", "dontexist.zip", ""),
		},
		{
			name: "invalid request to get metadata of a file with whitespaces as the name",
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
			m, err := database.GetFileMetadata(tt.fid)
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

func TestDatabase_UpdateFileMetadata(t *testing.T) {
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

	database := &database.Database{
		DB:  db,
		Log: logger,
	}

	populateDataset(database, t)

	tests := []struct {
		name     string
		fid      *v1.FileID
		metadata map[string]string
		errMsg   string
	}{
		{
			name: "update file metadata",
			fid:  &v1.FileID{Data: &v1.FileID_Name{Name: "reports.zip"}},
			metadata: map[string]string{
				"type":    "report",
				"version": "latest",
			},
		},
		{
			name: "update file metadata with same metadata",
			fid:  &v1.FileID{Data: &v1.FileID_Name{Name: "reports.zip"}},
			metadata: map[string]string{
				"type":    "report",
				"version": "latest",
			},
			errMsg: "connot update file metadata, as metadata of latest file and update request is same",
		},
		{
			name: "invalid fileId",
			fid:  &v1.FileID{Data: &v1.FileID_Name{Name: "invalid file"}},
			metadata: map[string]string{
				"type":    "report",
				"version": "latest",
			},
			errMsg: "no file found for provided_name:",
		},
		{
			name:     "empty metadata",
			fid:      &v1.FileID{Data: &v1.FileID_Name{Name: "reports.zip"}},
			metadata: map[string]string{},
			errMsg:   "nil arguments received: metadata: map[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := database.UpdateFileMetadata(tt.fid, tt.metadata)
			if err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message: %v, instead got: %v", tt.errMsg, err.Error())
				}
			} else if len(tt.errMsg) > 0 {
				t.Errorf("Expected error: %v was never received!", tt.errMsg)
			} else {
				fileMeta, err := database.GetFileMetadata(tt.fid)
				if err != nil {
					t.Errorf("Unexpected error while fetching file metadata: %v", err)
				}
				if len(tt.metadata) == len(fileMeta.FileMetadata) {
					match := matchMetadata(fileMeta.FileMetadata, tt.metadata)
					if !match {
						t.Errorf("Expected metadata does not match with updated")
					}
				} else {
					t.Errorf("Expected length of updated metadata does not match, Expected: %v Got %v", len(tt.metadata), len(fileMeta.FileMetadata))
				}
			}

		})
	}
}

func TestDatabase_CleanTombstones(t *testing.T) {
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

	database := &database.Database{
		DB:  db,
		Log: logger,
	}

	populateDataset(database, t)
	tests := []struct {
		name     string
		before   timestamppb.Timestamp
		purgeAll bool
		fids     []*v1.FileID
		errMsg   string
	}{
		{
			name:     "clear file contents",
			before:   timestamppb.Timestamp{Seconds: int64(before + 1)},
			purgeAll: false,
			fids: []*v1.FileID{
				{Data: &v1.FileID_Name{Name: "delete1.txt"}},
			},
		},
		{
			name:     "clear file contents",
			before:   timestamppb.Timestamp{Seconds: int64(after + 1)},
			purgeAll: false,
			fids: []*v1.FileID{
				{
					Data: &v1.FileID_Name{
						Name: "delete1.txt",
					},
				},
				{
					Data: &v1.FileID_Name{
						Name: "delete2.txt",
					},
				},
			},
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			fids, err := database.CleanTombstones(&tests[i].before, tests[i].purgeAll)
			if err != nil {
				if !strings.Contains(err.Error(), tests[i].errMsg) {
					t.Errorf("Expected error message: %v, instead got: %v", tests[i].errMsg, err.Error())
				}
			} else if len(tests[i].errMsg) > 0 {
				t.Errorf("Expected error: %v was never received!", tests[i].errMsg)
			}

			if len(fids) != len(tests[i].fids) {
				t.Errorf("Expected FId list was never received! \n Expected: %v \n Got: %v ", tests[i].fids, fids)
			}

			if !reflect.DeepEqual(fids, tests[i].fids) {
				t.Errorf("Expected FId list was never received! \n Expected: %v \n Got: %v ", tests[i].fids, fids)
			}
		})
	}
}

// populateDataset populates database with the files needed for testing
func populateDataset(database *database.Database, t *testing.T) {
	deleteFID := &v1.FileID{
		Data: &v1.FileID_Name{
			Name: "delete.txt",
		},
	}
	deleteFID1 := &v1.FileID{
		Data: &v1.FileID_Name{
			Name: "delete1.txt",
		},
	}
	deleteFID2 := &v1.FileID{
		Data: &v1.FileID_Name{
			Name: "delete2.txt",
		},
	}
	reportsFID := &v1.FileID{
		Data: &v1.FileID_Name{
			Name: "reports.zip",
		},
	}
	marketplaceFID := &v1.FileID{
		Data: &v1.FileID_Name{
			Name: "marketplace_report.zip",
		},
	}

	files := []v1.FileInfo{
		{
			FileId:          reportsFID,
			Size:            2000,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version": "2",
				"type":    "report",
			},
		},
		{
			FileId:          marketplaceFID,
			Size:            200,
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
		{
			FileId:      deleteFID,
			Size:        1500,
			Compression: false,
			Metadata: map[string]string{
				"description": "file marked for deletion",
			},
		},
		{
			FileId:      deleteFID1,
			Size:        1500,
			Compression: false,
			Metadata: map[string]string{
				"description": "file marked for deletion, and purged file content",
			},
		},
		{
			FileId:      deleteFID2,
			Size:        1500,
			Compression: false,
			Metadata: map[string]string{
				"description": "file marked for deletion, and purged file content",
			},
		},
	}
	// Upload files to mock server
	for i := range files {
		b := []byte(files[i].GetFileId().GetName())
		bs := make([]byte, (files[i].Size - uint32(len(b))))
		bs = append(bs, b...)
		files[i].Size = uint32(len(bs))
		dbErr := database.SaveFile(&files[i], bs)
		if dbErr != nil {
			t.Fatalf("Couldn't save file due to:%v", dbErr)
		}
		time.Sleep(1 * time.Second)
	}

	database.TombstoneFile(deleteFID)

	// update created_at for files
	setCreatedAt(reportsFID.GetName(), before, database)
	setCreatedAt(marketplaceFID.GetName(), after, database)
	setDeletedAt(deleteFID1.GetName(), before, database)
	setTombstone(deleteFID1.GetName(), before, database)
	setTombstone(deleteFID2.GetName(), after, database)
	time.Sleep(1 * time.Second)
}

// setCreatedAt modifies the created date for provided file
func setCreatedAt(fname string, cat int, d *database.Database) {
	d.DB.Model(&models.Metadata{}).
		Where("provided_name = ?", fname).
		Update("created_at", cat)
}

// setDeletedAt modifies deleted_at for provided file
func setDeletedAt(fname string, del int, d *database.Database) {
	d.DB.Model(&models.Metadata{}).
		Where("provided_name = ?", fname).
		Update("deleted_at", del)
}

// setTombstone modifies clean_tombstone_set_at
func setTombstone(fname string, tomb int, d *database.Database) {
	m := &models.Metadata{}
	d.DB.Model(&m).
		Where("provided_name = ?", fname).
		Update("clean_tombstone_set_at", tomb)
}

//  matchMetadata return true if expected metadata and retrieved metadata is same else false
func matchMetadata(fms []models.FileMetadata, fmMap map[string]string) bool {

	if len(fms) != len(fmMap) {
		return false
	}

	oldFms := make(map[string]string)
	for _, fm := range fms {
		oldFms[fm.Key] = fm.Value
	}
	eq := reflect.DeepEqual(oldFms, fmMap)

	return eq
}
