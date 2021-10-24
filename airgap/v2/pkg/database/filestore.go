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
	"context"
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
	List(ctx context.Context, opts ...ListOption) (files []models.StoredFile, nextPageToken string, err error)
	Get(ctx context.Context, id string) (*models.StoredFile, error)
	GetByFileKey(ctx context.Context, fileKey modelsv2.StoredFileKey) (*models.StoredFile, error)
	Save(ctx context.Context, file *models.StoredFile) (id string, err error)
	Delete(ctx context.Context, id string, permanent bool) error
	Download(ctx context.Context, id string) (*models.StoredFile, error)
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

// SELECT File.Checksum,File.Size,File.MimeType,`File`.`id` AS `File__id`,`File`.`created_at` AS `File__created_at`,`File`.`updated_at` AS `File__updated_at`,`File`.`deleted_at` AS `File__deleted_at`,`File`.`file_id` AS `File__file_id`,`File`.`checksum` AS `File__checksum`,`File`.`size` AS `File__size`,`File`.`mime_type` AS `File__mime_type`,`File`.`content` AS `File__content` FROM `stored_files`
// LEFT JOIN `stored_file_contents` `File` ON `stored_files`.`id` = `File`.`file_id`
// WHERE `stored_files`.`id` = 1 AND `stored_files`.`deleted_at` IS NULL ORDER BY `stored_files`.`id` LIMIT 1

func (d *fileStore) Get(ctx context.Context, id string) (*models.StoredFile, error) {
	idInt, err := modelsv2.ConvertStrToUint(id)

	if err != nil {
		return nil, err
	}

	db := d.DB.WithContext(ctx)
	file := models.StoredFile{}

	if err := db.Unscoped().
		Preload("FileMetadata", func(db *gorm.DB) *gorm.DB {
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

func (d *fileStore) GetByFileKey(ctx context.Context, fileKey modelsv2.StoredFileKey) (*models.StoredFile, error) {

	db := d.DB.WithContext(ctx)
	file := models.StoredFile{}

	if err := db.Unscoped().
		Preload("FileMetadata", func(db *gorm.DB) *gorm.DB {
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

func (d *fileStore) Save(ctx context.Context, file *models.StoredFile) (id string, err error) {
	if file != nil {
		err = errors.WrapWithDetails(ErrInvalidInput, "type", "file is nil")
	}

	if file.File.Content == nil {
		err = errors.WrapWithDetails(ErrInvalidInput, "type", "no content provided")
	}

	db := d.DB.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: true})

	if file.ID == 0 {
		foundFile := &models.StoredFile{}

		err := db.Where(&models.StoredFile{
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
	return
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
		if err := tx.Where("file_id = ?", file.ID).Delete(&modelsv2.StoredFileContent{}).Error; err != nil {
			return err
		}

		if err := tx.Where("file_id = ?", file.ID).Delete(&modelsv2.StoredFileMetadata{}).Error; err != nil {
			return err
		}

		return tx.Delete(file).Error
	})

	return
}

func (d *fileStore) List(ctx context.Context, opts ...ListOption) (files []models.StoredFile, nextPageToken string, err error) {
	listOpts := (&ListOptions{}).ApplyOptions(opts)
	err = d.DB.WithContext(ctx).
		Scopes(listOpts.scopes()...).
		Preload("FileMetadata", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Preload("File", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Omit("File.Content").
		Order("created_at desc").
		Find(&files).Error

	if len(files) == listOpts.Pagination.PageSize+1 {
		nextPageToken = fmt.Sprintf("%d", listOpts.Pagination.Page+1)

		// We purposedly limit to pagesize + 1 to check if
		// there is a page past the last number.
		// We trim to the page size to prevent having too many
		// results on each call
		files = files[:len(files)-1]
	}

	return
}

func (d *fileStore) Download(ctx context.Context, id string) (file *models.StoredFile, err error) {
	file = &models.StoredFile{}
	err = d.WithContext(ctx).
		Unscoped().
		Preload("FileMetadata", func(db *gorm.DB) *gorm.DB {
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

	tx1 := d.WithContext(ctx).Unscoped().Select(clause.Associations).Where("deleted_at < ?", now).Delete(models.StoredFile{})
	if tx1.Error != nil {
		return 0, tx1.Error
	}

	tx2 := d.WithContext(ctx).Unscoped().Select(clause.Associations).Where("deleted_at < ?", now).Delete(models.StoredFileContent{})
	if tx2.Error != nil {
		return 0, tx2.Error
	}

	tx3 := d.WithContext(ctx).Unscoped().Select(clause.Associations).Where("deleted_at < ?", now).Delete(models.StoredFileMetadata{})
	if tx3.Error != nil {
		return 0, tx3.Error
	}

	return tx1.RowsAffected + tx2.RowsAffected + tx3.RowsAffected, nil
}
