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

package modelsv2

import (
	"crypto/sha256"
	"fmt"

	"gorm.io/gorm"
)

type StoredFileMetadata struct {
	gorm.Model

	FileID uint   `gorm:"uniqueIndex:idx_stored_file_metadata"`
	Key    string `gorm:"uniqueIndex:idx_stored_file_metadata"`
	Value  string
}

type StoredFileContent struct {
	gorm.Model

	FileID uint `gorm:"uniqueIndex"`

	Checksum string
	Size     int
	MimeType string

	Content []byte
}

type StoredFile struct {
	gorm.Model

	Name       string `gorm:"uniqueIndex:idx_stored_file-name"`
	Source     string `gorm:"uniqueIndex:idx_stored_file-name"`
	SourceType string `gorm:"uniqueIndex:idx_stored_file-name"`

	File     StoredFileContent    `gorm:"foreignKey:FileID"`
	Metadata []StoredFileMetadata `gorm:"foreignKey:FileID"`
}

type StoredFileKey struct {
	gorm.Model

	Name       string
	Source     string
	SourceType string
}

func (f *StoredFileContent) BeforeSave(tx *gorm.DB) (err error) {
	if len(f.Content) == 0 {
		return nil
	}

	{ // calculate Size
		f.Size = len(f.Content)
	}

	{ //calculate checksum
		sum := sha256.Sum256(f.Content)
		f.Checksum = fmt.Sprintf("%x", sum)
	}

	return
}
