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

package scheduler

import (
	"log"
	"os"
	"reflect"
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

func TestScheduler_handler(t *testing.T) {
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
	sfg := SchedulerConfig{
		Log: logger,
		Fs:  database,
	}
	populateDataset(database, t)

	tests := []struct {
		name   string
		after  int
		purge  bool
		fids   []*v1.FileID
		errMsg string
	}{
		{
			name:  "clean files marked for deletion",
			after: 20,
			purge: false,
			fids: []*v1.FileID{
				{Data: &v1.FileID_Name{Name: "delete.txt"}},
				{Data: &v1.FileID_Name{Name: "delete1.txt"}},
			},
			errMsg: "",
		},
		{
			name:  "clean files marked for deletion",
			after: 10,
			purge: true,
			fids: []*v1.FileID{
				{Data: &v1.FileID_Name{Name: "delete.txt"}},
				{Data: &v1.FileID_Name{Name: "delete1.txt"}},
				{Data: &v1.FileID_Name{Name: "delete2.txt"}},
			},
			errMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fids, err := sfg.handler(tt.after, tt.purge)
			if err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message: %v, instead got: %v", tt.errMsg, err.Error())
				}
			} else if len(tt.errMsg) > 0 {
				t.Errorf("Expected error: %v was never received!", tt.errMsg)
			}
			if !reflect.DeepEqual(fids, tt.fids) {
				t.Errorf("Expected fileIds: %v, instead got %v", tt.fids, fids)
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

	files := []v1.FileInfo{
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

	// update fields for seed data
	setTombstone(deleteFID.GetName(), 30, database)
	setTombstone(deleteFID1.GetName(), 20, database)
	setTombstone(deleteFID2.GetName(), 10, database)
	time.Sleep(1 * time.Second)
}

// setTombstone modifies clean_tombstone_set_at
func setTombstone(fname string, before int, d *database.Database) {
	m := &models.Metadata{}
	now := time.Now()
	t1 := now.Add(time.Duration(-before) * time.Hour).Unix()

	d.DB.Model(&m).
		Where("provided_name = ?", fname).
		Update("clean_tombstone_set_at", t1)
}
