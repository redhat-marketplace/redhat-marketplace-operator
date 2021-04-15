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

package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Metadata struct {
	ID                  string `gorm:"primaryKey"`
	ProvidedId          string
	ProvidedName        string
	Size                uint32
	Compression         bool
	CompressionType     string
	CleanTombstoneSetAt int64
	CreatedAt           int64
	DeletedAt           int64
	FileID              string
	File                File
	FileMetadata        []FileMetadata
}

func (m *Metadata) BeforeCreate(tx *gorm.DB) (err error) {
	m.ID = uuid.NewString()
	return
}
