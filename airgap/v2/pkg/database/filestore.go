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
	// "crypto/sha256"
	// "encoding/hex"
	// "fmt"
	// "reflect"
	// "strings"
	"fmt"
	"io"
	"time"

	// "unicode"
	// "google.golang.org/protobuf/types/known/timestamppb"
	// "gorm.io/gorm/clause"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	models "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	modelsv2 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type StoredFileStore interface {
	List(opts ...ListOption) ([]models.StoredFile, error)
	Get(string) (*models.StoredFile, error)
	Save(*models.StoredFile) (id string, err error)
	Delete(string) error
	Download(string) (*models.StoredFile, error)
	CleanTombstones() (int64, error)
}

func New(db *gorm.DB, config FileStoreConfig) (StoredFileStore, io.Closer) {
	store := &fileStore{
		DB:     db,
		Log:    logf.Log.WithName("file_store"),
		config: config,
	}
	return store, store
}

type FileStoreConfig struct {
	CleanupAfter time.Duration
}

type fileStore struct {
	*gorm.DB
	Log logr.Logger

	config FileStoreConfig
}

type SortOrder struct {
	Key   string
	Order string
}

type Condition struct {
	Key      string
	Operator string
	Value    string
}

type metaDataQuery struct {
	MetaString string
	MetaValue  []interface{}
}

const (
	ErrInvalidInput = errors.Sentinel("invalid input")
)

func (d *fileStore) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		d.Log.Error(err, "couldn't close db")
		return err
	}
	return sqlDB.Close()
}

func (d *fileStore) Get(id string) (*models.StoredFile, error) {
	idInt, err := modelsv2.ConvertStrToUint(id)

	if err != nil {
		return nil, err
	}

	file := models.StoredFile{}

	if err := d.DB.Preload("FileMetadata").First(&file, idInt).Error; err != nil {
		return nil, err
	}

	return &file, nil
}

func (d *fileStore) Save(file *models.StoredFile) (id string, err error) {
	if file != nil {
		err = errors.WrapWithDetails(ErrInvalidInput, "type", "file is nil")
	}

	if file.File.Content == nil {
		err = errors.WrapWithDetails(ErrInvalidInput, "type", "no content provided")
	}

	err = d.DB.Session(&gorm.Session{FullSaveAssociations: true}).Save(file).Error
	id = fmt.Sprintf("%d", file.ID)
	return
}

func (d *fileStore) Delete(id string) (err error) {
	var file *modelsv2.StoredFile
	file, err = d.Get(id)

	if err != nil {
		return err
	}
	if file == nil {
		return
	}

	err = d.DB.Select(clause.Associations).Delete(file).Error
	return
}

func (d *fileStore) List(opts ...ListOption) (files []models.StoredFile, err error) {
	listOpts := (&ListOptions{}).ApplyOptions(opts)
	err = d.DB.Scopes(listOpts.scopes()...).Find(&files).Error
	return
}

func (d *fileStore) Download(id string) (file *models.StoredFile, err error) {
	file = &models.StoredFile{}
	err = d.Unscoped().Preload("File").Preload("FileMetadata").First(file, id).Error
	return
}

func (d *fileStore) CleanTombstones() (int64, error) {
	now := time.Now()
	now = now.Add(-d.config.CleanupAfter)

	d.Log.Info("cleaning up all files older than", "now", now.String())

	tx1 := d.Unscoped().Select(clause.Associations).Where("deleted_at < ?", now).Delete(models.StoredFile{})
	if tx1.Error != nil {
		return 0, tx1.Error
	}

	tx2 := d.Unscoped().Select(clause.Associations).Where("deleted_at < ?", now).Delete(models.StoredFileContent{})
	if tx2.Error != nil {
		return 0, tx2.Error
	}

	tx3 := d.Unscoped().Select(clause.Associations).Where("deleted_at < ?", now).Delete(models.StoredFileMetadata{})
	if tx3.Error != nil {
		return 0, tx3.Error
	}

	return tx1.RowsAffected + tx2.RowsAffected + tx3.RowsAffected, nil
}
