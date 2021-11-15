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
	gormigrate "github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	modelsv1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v1"
	models "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	"gorm.io/gorm"
)

var (
	migrations = []*gormigrate.Migration{
		// create v1 tables
		{
			ID: "202109010000",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(modelsv1.File{}, modelsv1.FileMetadata{}, modelsv1.Metadata{})
			},
			Rollback: func(tx *gorm.DB) (err error) {
				if err = tx.Migrator().DropTable("files"); err != nil {
					return
				}

				if err = tx.Migrator().DropTable("file_metadatas"); err != nil {
					return
				}

				return tx.Migrator().DropTable("metadatas")
			},
		},
		// create v2 tables and migrate
		{
			ID: "202110160004",
			Migrate: func(tx *gorm.DB) (err error) {
				if err = tx.AutoMigrate(models.StoredFileContent{}, models.StoredFileMetadata{}, models.StoredFile{}); err != nil {
					return
				}

				{
					batchLimit := 10
					oldFiles := []modelsv1.Metadata{}
					result := tx.Unscoped().
						Preload("File").
						Preload("FileMetadata").
						FindInBatches(&oldFiles, batchLimit, func(_ *gorm.DB, batch int) error {
							uniqueID := uuid.NewString()
							newFiles := []*models.StoredFile{}

							for _, file := range oldFiles {
								newFile := &models.StoredFile{
									Name:       file.ProvidedName,
									Source:     "redhat-marketplace",
									SourceType: "migrate-202110160004-" + uniqueID,
									File: models.StoredFileContent{
										Content:  file.File.Content,
										MimeType: "application/gzip",
									},
									Metadata: []models.StoredFileMetadata{},
								}

								for i := range file.FileMetadata {
									data := file.FileMetadata[i]
									newFile.Metadata = append(newFile.Metadata, models.StoredFileMetadata{
										Key:   data.Key,
										Value: data.Value,
									})
								}

								newFiles = append(newFiles, newFile)
							}

							if len(newFiles) > 0 {
								batchErr := tx.Create(&newFiles).Error
								if err != nil {
									return batchErr
								}
							}

							// returns error will stop future batches
							return nil
						})

					if err = result.Error; err != nil {
						return
					}
				}

				return
			},
			Rollback: func(tx *gorm.DB) (err error) {
				if err = tx.Migrator().DropTable("stored_files"); err != nil {
					return
				}

				if err = tx.Migrator().DropTable("stored_file_metadata"); err != nil {
					return
				}

				if err = tx.Migrator().DropTable("stored_file_contents"); err != nil {
					return
				}

				return
			},
		},
		// drop v1 tables
		{
			ID: "20211110000",
			Migrate: func(tx *gorm.DB) (err error) {
				if err = tx.Migrator().DropTable("files"); err != nil {
					return
				}

				if err = tx.Migrator().DropTable("metadata"); err != nil {
					return
				}

				if err = tx.Migrator().DropTable("file_metadata"); err != nil {
					return
				}

				if err = tx.Migrator().DropTable("file_contents"); err != nil {
					return
				}

				if err = tx.AutoMigrate(
					models.StoredFileContent{},
					models.StoredFileMetadata{},
					models.StoredFile{}); err != nil {
					return
				}

				return
			},
		},
	}
)

func migrator(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, migrations)
}

func Migrate(db *gorm.DB) error {
	m := migrator(db)

	if err := m.Migrate(); err != nil {
		return err
	}

	if err := db.AutoMigrate(models.StoredFile{}, models.StoredFileContent{}, models.StoredFileMetadata{}); err != nil {
		return err
	}

	return nil
}
