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

//go:build !linux
// +build !linux

package dqlite

import (
	"io"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type DatabaseConfig struct {
	Name    string
	Dir     string
	Url     string
	Join    *[]string
	Verbose bool
	Log     logr.Logger
	gormDB  *gorm.DB
	CACert  string
	TLSCert string
	TLSKey  string

	closer io.Closer
}

// InitDB initializes the GORM connection and returns a connected struct
func (dc *DatabaseConfig) InitDB(
	cleanUpAfter time.Duration,
) (database.StoredFileStore, error) {
	dc.Log.Info("WARNING ---- running sqlite mode because os is not linux ---- WARNING")
	p := filepath.Join(dc.Dir, dc.Name)
	db, err := gorm.Open(sqlite.Open(p), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	database, closer := database.New(db, database.FileStoreConfig{CleanupAfter: cleanUpAfter})
	dc.gormDB = db
	dc.closer = closer
	return database, err
}

// TryMigrate  performs database migration
func (dc *DatabaseConfig) TryMigrate() error {
	return database.Migrate(dc.gormDB)
}

// IsLeader returns true if running node is leader
func (dc *DatabaseConfig) IsLeader() (bool, error) {
	dc.Log.Info("WARNING ---- running sqlite mode because os is not linux ---- WARNING")
	return true, nil
}

// Close ensures all responsibilities for the node are handled gracefully on exit
func (dc *DatabaseConfig) Close() {
	if dc != nil {
		dc.closer.Close()
	}
}
