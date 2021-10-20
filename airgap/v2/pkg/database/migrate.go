package database

import (
	"log"

	gormigrate "github.com/go-gormigrate/gormigrate/v2"
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
					oldFiles := []modelsv1.Metadata{}
					newFiles := []models.StoredFile{}
					result := tx.Preload("File").Preload("FileMetadata").FindInBatches(&oldFiles, 10, func(tx *gorm.DB, batch int) error {
						for _, file := range oldFiles {
							newFile := models.StoredFile{
								Name:       file.ProvidedName,
								Source:     "redhat-marketplace",
								SourceType: "report",
								File: models.StoredFileContent{
									Content:  file.File.Content,
									MimeType: "application/tar+gz",
								},
								FileMetadata: []models.StoredFileMetadata{},
							}

							for i := range file.FileMetadata {
								data := file.FileMetadata[i]
								newFile.FileMetadata = append(newFile.FileMetadata, models.StoredFileMetadata{
									Key:   data.Key,
									Value: data.Value,
								})
							}

							newFiles = append(newFiles, newFile)
						}

						// returns error will stop future batches
						return nil
					})

					if err = result.Error; err != nil {
						return
					}

					if len(newFiles) > 0 {
						err := tx.Save(newFiles).Error
						if err != nil {
							return err
						}
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

				if err = tx.Migrator().DropTable("file_contents"); err != nil {
					return
				}

				if err = tx.AutoMigrate(models.StoredFileContent{}, models.StoredFileMetadata{}, models.StoredFile{}); err != nil {
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
		log.Fatalf("Could not migrate: %v", err)
	}

	if err := db.AutoMigrate(models.StoredFile{}, models.StoredFileContent{}, models.StoredFileMetadata{}); err != nil {
		log.Fatalf("Could not migrate: %v", err)
	}

	return nil
}
