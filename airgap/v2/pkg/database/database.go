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

package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/canonical/go-dqlite/app"
	"github.com/canonical/go-dqlite/client"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/driver/dqlite"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models"
	"gorm.io/gorm"
)

type Database struct {
	DB       *gorm.DB
	dqliteDB *sql.DB
	app      *app.App
	Log      logr.Logger
}

type DatabaseConfig struct {
	Name    string
	Dir     string
	Url     string
	Join    *[]string
	Verbose bool
	Log     logr.Logger
}

// Initialize the GORM connection and return connected struct
func (dc *DatabaseConfig) InitDB() (*Database, error) {
	database, err := dc.initDqlite()
	if err != nil {
		return nil, err
	}

	dqliteDialector := dqlite.Open(database.dqliteDB)
	database.DB, err = gorm.Open(dqliteDialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto migrate models
	database.DB.AutoMigrate(&models.File{}, &models.FileMetadata{}, &models.Metadata{})
	database.Log = dc.Log
	return database, err
}

// Initialize the underlying dqlite database and populate a *Database object with the dqlite connection and app
func (dc *DatabaseConfig) initDqlite() (*Database, error) {
	dc.Dir = filepath.Join(dc.Dir, dc.Url)
	if err := os.MkdirAll(dc.Dir, 0755); err != nil {
		return nil, errors.Wrapf(err, "can't create %s", dc.Dir)
	}
	logFunc := func(l client.LogLevel, format string, a ...interface{}) {
		if !dc.Verbose {
			return
		}
		log.Printf(fmt.Sprintf("%s: %s\n", l.String(), format), a...)
	}

	app, err := app.New(dc.Dir, app.WithAddress(dc.Url), app.WithCluster(*dc.Join), app.WithLogFunc(logFunc))
	if err != nil {
		return nil, err
	}

	if err := app.Ready(context.Background()); err != nil {
		return nil, err
	}

	conn, err := app.Open(context.Background(), dc.Name)
	if err != nil {
		return nil, err
	}

	return &Database{dqliteDB: conn, app: app}, conn.Ping()
}

func (d *Database) SaveFile(finfo *v1.FileInfo, bs []byte) error {
	// Create a slice of file metadata models
	var fms []models.FileMetadata
	m := finfo.GetMetadata()
	for k, v := range m {
		fm := models.FileMetadata{
			Key:   k,
			Value: v,
		}
		fms = append(fms, fm)
	}

	// Create metadata along with associations
	metadata := models.Metadata{
		ProvidedId:      finfo.GetFileId().GetId(),
		ProvidedName:    finfo.GetFileId().GetName(),
		Size:            finfo.GetSize(),
		Compression:     finfo.GetCompression(),
		CompressionType: finfo.GetCompressionType(),
		File: models.File{
			Content: bs,
		},
		FileMetadata: fms,
	}
	err := d.DB.Create(&metadata).Error
	if err != nil {
		d.Log.Error(err, "Failed to save model")
		return err
	}

	d.Log.Info(fmt.Sprintf("File of size: %v saved with id: %v", metadata.Size, metadata.FileID))
	return nil
}

// Close connection to the database and perform context handover
func (d *Database) Close() {
	if d != nil {
		d.Log.Info("Attempting graceful shutdown and handover")
		d.dqliteDB.Close()
		d.app.Handover(context.Background())
		d.app.Close()
	}
}
