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
	"strconv"
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
	ErrNotFound     = errors.Sentinel("not found")
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
		Model(&file).
		Preload("Metadata").
		Preload("File", func(db *gorm.DB) *gorm.DB {
			return db.Omit("content")
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
		Model(&file).
		Preload("Metadata").
		Preload("File", func(db *gorm.DB) *gorm.DB {
			return db.Omit("content")
		}).
		Where(modelsv2.StoredFile{
			Name:       fileKey.Name,
			Source:     fileKey.Source,
			SourceType: fileKey.SourceType,
		}).
		First(&file).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &file, nil
}

func (d *fileStore) Save(ctx context.Context, file *modelsv2.StoredFile) (string, error) {
	if file == nil {
		err := errors.Wrap(ErrInvalidInput, "file is nil")
		return "", err
	}

	db := d.DB.WithContext(ctx)
	foundFile := &modelsv2.StoredFile{}

	// attempt to find file
	if file.ID != 0 {
		var err error
		foundFile, err = d.Get(ctx, strconv.Itoa(int(file.ID)))
		if err != nil && !errors.Is(err, ErrNotFound) {
			return "", err
		}
	} else if file.Name != "" &&
		file.Source != "" &&
		file.SourceType != "" {
		var err error
		foundFile, err = d.GetByFileKey(ctx, &modelsv2.StoredFileKey{
			Name:       file.Name,
			Source:     file.Source,
			SourceType: file.SourceType,
		})
		if err != nil && !errors.Is(err, ErrNotFound) {
			return "", err
		}
	}

	//notFound create it
	if foundFile == nil || foundFile.ID == 0 {
		err := d.DB.WithContext(ctx).
			Create(file).Error
		return fmt.Sprintf("%d", file.ID), err
	}

	// Explicitly set DeletedAt.Valid false to prevent UPDATE of deleted_at
	// A DeletedAt.Time = 0 is set in the struct produced by a Get(), even if deleted_at is NULL
	// A default DeletedAt struct will also trigger and UPDATE of deleted_at from NULL to 0
	// This will cause LIST to fail, and the file to be tombstoned
	file.DeletedAt = gorm.DeletedAt{Valid: false}

	for i := range file.Metadata {
		metadata := &file.Metadata[i]

		foundMetadata := &modelsv2.StoredFileMetadata{}
		err := db.Model(&modelsv2.StoredFileMetadata{}).
			Where("file_id == ? AND key == ?", foundFile.ID, metadata.Key).
			First(foundMetadata).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			err = errors.WithStack(err)
			return "", err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			metadata.FileID = foundFile.ID
			err := db.Create(metadata).Error
			if err != nil {
				err = errors.WithStack(err)
				return "", err
			}
		} else {
			err := db.Model(foundMetadata).Updates(metadata).Error
			if err != nil {
				err = errors.WithStack(err)
				return "", err
			}
		}
	}

	content := &file.File
	if len(content.Content) != 0 {
		if content.ID != 0 {
			foundContent := &foundFile.File
			err := db.Model(foundContent).
				Updates(content).Error

			if err != nil {
				err = errors.WithStack(err)
				return "", err
			}
		} else {
			content.FileID = foundFile.ID
			err := db.Create(content).Error

			if err != nil {
				err = errors.WithStack(err)
				return "", err
			}
		}
	}

	err := d.DB.WithContext(ctx).
		Omit("Content").
		Model(foundFile).
		Where("id = ?", foundFile.ID).
		Updates(*file).Error
	id := fmt.Sprintf("%d", foundFile.ID)
	return id, err
}

func (d *fileStore) Delete(ctx context.Context, id string, permanent bool) (err error) {
	db := d.DB.WithContext(ctx)

	if permanent {
		db = db.Unscoped().Select(clause.Associations)
	}

	idInt, err := modelsv2.ConvertStrToUint(id)

	if err != nil {
		return err
	}

	err = db.Model(&modelsv2.StoredFile{}).
		Delete(&modelsv2.StoredFile{}, idInt).Error

	return err
}

func (d *fileStore) List(ctx context.Context, opts ...ListOption) (files []modelsv2.StoredFile, nextPageToken string, err error) {
	listOpts := *(&ListOptions{}).ApplyOptions(opts)

	listOpts2 := listOpts
	listOpts2.Pagination = nil

	db := d.DB.WithContext(ctx)

	idQuery := db.Model(&modelsv2.StoredFile{}).
		Unscoped().
		Scopes(listOpts2.scopes()...).
		Joins("left join stored_file_metadata on stored_files.id = stored_file_metadata.file_id").
		Joins("left join stored_file_contents on stored_file_contents.file_id = stored_files.id").
		Distinct("stored_files.id")

	// reset filters for this
	listOpts.Filters = []*fileserver.Filter{}

	query := db.Model(&modelsv2.StoredFile{}).
		Scopes(listOpts.scopes()...).
		Where("stored_files.id in (?)", idQuery).
		Preload("Metadata").
		Preload("File").
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
		Preload("Metadata").
		Preload("File").
		First(file, id).Error
	return
}

func (d *fileStore) CleanTombstones(ctx context.Context) (int64, error) {
	now := time.Now()
	now = now.Add(-d.config.CleanupAfter)

	d.Log.Info("cleaning up all files older than", "now", now.String())

	q := d.WithContext(ctx).
		Unscoped().
		Model(&modelsv2.StoredFile{}).
		Where("deleted_at < ?", now).
		Select("id")

	var rowsAffected int64
	err := d.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&modelsv2.StoredFileContent{}).
			Where("file_id in (?)", q).
			Delete(&[]modelsv2.StoredFileContent{}).Error; err != nil {
			return err
		}

		if err := tx.Model(&modelsv2.StoredFileMetadata{}).
			Where("file_id in (?)", q).
			Delete(&[]modelsv2.StoredFileMetadata{}).Error; err != nil {
			return err
		}

		tx1 := tx.Unscoped().
			Model(&modelsv2.StoredFile{}).
			Where("id in (?)", q).
			Delete(&[]modelsv2.StoredFile{})
		if err := tx1.Error; err != nil {
			return err
		}

		rowsAffected = tx1.RowsAffected
		return nil
	})

	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}
