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
	"strings"
	"testing"

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
