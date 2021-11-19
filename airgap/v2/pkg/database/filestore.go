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
	"fmt"
	"io"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1/fileserver"
	modelsv2 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type StoredFileStore interface {
	List(ctx context.Context, opts ...ListOption) (files []modelsv2.StoredFile, nextPageToken string, err error)
	Get(ctx context.Context, id string) (*modelsv2.StoredFile, error)
	GetByFileKey(ctx context.Context, fileKey *modelsv2.StoredFileKey) (*modelsv2.StoredFile, error)
	Save(ctx context.Context, file *modelsv2.StoredFile) (id string, err error)
	Delete(ctx context.Context, id string, permanent bool) error
	Download(ctx context.Context, id string) (*modelsv2.StoredFile, error)
	CleanTombstones(ctx context.Context) (int64, error)
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

func (d *fileStore) Get(ctx context.Context, id string) (*modelsv2.StoredFile, error) {
	idInt, err := modelsv2.ConvertStrToUint(id)

	if err != nil {
		return nil, err
	}

	db := d.DB.WithContext(ctx)
	file := modelsv2.StoredFile{}

	if err := db.Unscoped().
		Preload("Metadata", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Preload("File", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped().Omit("content")
		}).
		Find(&file, idInt).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

func (d *fileStore) GetByFileKey(ctx context.Context, fileKey *modelsv2.StoredFileKey) (*modelsv2.StoredFile, error) {
	if fileKey == nil {
		return nil, errors.New("filekey is nil")
	}

	db := d.DB.WithContext(ctx)
	file := modelsv2.StoredFile{}

	if err := db.Unscoped().
		Preload("Metadata", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Preload("File", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped().Omit("content")
		}).
		Where(&fileKey).
		First(&file).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

func (d *fileStore) Save(ctx context.Context, file *modelsv2.StoredFile) (id string, err error) {
	if file == nil {
		err = errors.WrapWithDetails(ErrInvalidInput, "type", "file is nil")
		return
	}

	if len(file.File.Content) == 0 {
		err = errors.WrapWithDetails(ErrInvalidInput, "type", "no content provided")
		return
	}

	db := d.DB.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: true})

	if file.ID == 0 {
		foundFile := modelsv2.StoredFile{}

		err = db.Where(&modelsv2.StoredFile{
			Name:       file.Name,
			Source:     file.Source,
			SourceType: file.SourceType,
		}).Find(&foundFile).Error

		if err != nil {
			return "", err
		}

		file.ID = foundFile.ID
	}

	err = db.Save(file).Error
	id = fmt.Sprintf("%d", file.ID)
	return id, err
}

func (d *fileStore) Delete(ctx context.Context, id string, permanent bool) (err error) {
	var file *modelsv2.StoredFile
	file, err = d.Get(ctx, id)

	if err != nil {
		return err
	}

	if file == nil {
		return
	}

	db := d.DB.WithContext(ctx)

	if permanent {
		db = db.Unscoped()
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if inErr := tx.Where("file_id = ?", file.ID).Delete(&modelsv2.StoredFileContent{}).Error; inErr != nil {
			return inErr
		}

		if inErr := tx.Where("file_id = ?", file.ID).Delete(&modelsv2.StoredFileMetadata{}).Error; inErr != nil {
			return inErr
		}

		return tx.Delete(file).Error
	})

	return err
}

func (d *fileStore) List(ctx context.Context, opts ...ListOption) (files []modelsv2.StoredFile, nextPageToken string, err error) {
	listOpts := *(&ListOptions{}).ApplyOptions(opts)

	listOpts2 := listOpts
	listOpts2.Pagination = nil

	idQuery := d.DB.Model(&modelsv2.StoredFileMetadata{}).
		Unscoped().
		Scopes(listOpts2.scopes()...).
		Joins("join stored_files on stored_files.id = stored_file_metadata.file_id").
		Joins("join stored_file_contents on stored_file_contents.file_id = stored_file_metadata.file_id").
		Distinct("stored_file_metadata.file_id")

	idQuery2 := *idQuery

	var count int64

	if err = idQuery2.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		return
	}

	if count == 0 {
		return
	}

	// reset filters for this
	listOpts.Filters = []*fileserver.Filter{}

	query := d.DB.WithContext(ctx).
		Scopes(listOpts.scopes()...).
		Preload("Metadata", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Preload("File", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Where("id in (?)", idQuery).
		Omit("File.Content").
		Order("stored_files.created_at desc").
		Find(&files)

	err = query.Error

	if len(files) == listOpts.Pagination.PageSize+1 {
		nextPageToken = fmt.Sprintf("%d", listOpts.Pagination.Page+1)

		// We purposely limit to pagesize + 1 to check if
		// there is a page past the last number.
		// We trim to the page size to prevent having too many
		// results on each call
		files = files[:len(files)-1]
	}

	return
}

func (d *fileStore) Download(ctx context.Context, id string) (file *modelsv2.StoredFile, err error) {
	file = &modelsv2.StoredFile{}
	err = d.WithContext(ctx).
		Unscoped().
		Preload("Metadata", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Preload("File", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		First(file, id).Error
	return
}

func (d *fileStore) CleanTombstones(ctx context.Context) (int64, error) {
	now := time.Now()
	now = now.Add(-d.config.CleanupAfter)

	d.Log.Info("cleaning up all files older than", "now", now.String())

	tx1 := d.WithContext(ctx).Unscoped().Select(clause.Associations).Where("deleted_at < ?", now).Delete(modelsv2.StoredFile{})
	if tx1.Error != nil {
		return 0, tx1.Error
	}

	tx2 := d.WithContext(ctx).Unscoped().Select(clause.Associations).Where("deleted_at < ?", now).Delete(modelsv2.StoredFileContent{})
	if tx2.Error != nil {
		return 0, tx2.Error
	}

	tx3 := d.WithContext(ctx).Unscoped().Select(clause.Associations).Where("deleted_at < ?", now).Delete(modelsv2.StoredFileMetadata{})
	if tx3.Error != nil {
		return 0, tx3.Error
	}

	return tx1.RowsAffected + tx2.RowsAffected + tx3.RowsAffected, nil
}
