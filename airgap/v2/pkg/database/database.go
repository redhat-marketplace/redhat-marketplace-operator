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
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/go-logr/logr"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FileStore interface {
	SaveFile(finfo *v1.FileInfo, bs []byte) error
	DownloadFile(finfo *v1.FileID) (*models.Metadata, error)
	ListFileMetadata([]*Condition, []*SortOrder) ([]models.Metadata, error)
}

type Database struct {
	DB  *gorm.DB
	Log logr.Logger
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

// SaveFile allows us to save a file along with it's metadata to the database
func (d *Database) SaveFile(finfo *v1.FileInfo, bs []byte) error {
	// Validating input data
	if finfo == nil || bs == nil {
		return fmt.Errorf("nil arguments received: finfo: %v bs: %v", finfo, bs)
	} else if finfo.GetFileId() == nil {
		return fmt.Errorf("file id struct is nil")
	} else if len(strings.TrimSpace(finfo.GetFileId().GetId())) == 0 && len(strings.TrimSpace(finfo.GetFileId().GetName())) == 0 {
		return fmt.Errorf("file id/name is blank")
	}

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

	d.Log.Info("Saved file", "size", metadata.Size, "id", metadata.FileID)
	return nil
}

// DownloadFile allows us to extract a file and it's metadata from the database provided it exists
func (d *Database) DownloadFile(finfo *v1.FileID) (*models.Metadata, error) {

	var meta models.Metadata

	fileid := strings.TrimSpace(finfo.GetId())
	filename := strings.TrimSpace(finfo.GetName())

	// Perform validations and query
	if len(fileid) != 0 {
		d.DB.Where("provided_id = ?", fileid).
			Order("created_at desc").
			Preload(clause.Associations).
			First(&meta)
	} else if len(filename) != 0 {
		d.DB.Where("provided_name = ?", filename).
			Order("created_at desc").
			Preload(clause.Associations).
			First(&meta)
	} else {
		return nil, fmt.Errorf("file id/name is blank")
	}

	if reflect.DeepEqual(meta, models.Metadata{}) {
		return nil, fmt.Errorf("no file found for provided_name: %v / provided_id: %v", filename, fileid)
	}
	d.Log.Info("Retreived file", "size", meta.Size, "id", meta.FileID)
	return &meta, nil
}

// ListFileMetadata allow us to fetch list of files and its metadata from database
func (d *Database) ListFileMetadata(conditionList []*Condition, sortOrderList []*SortOrder) ([]models.Metadata, error) {
	//Converts the keys from golang notation to the database notation i.e., HelloWorldGlobe -> hello_world_globe
	cleanKey := func(s string) string {
		output := []byte{}
		for id, c := range s {
			if unicode.IsUpper(c) && id != 0 {
				output = append(output, '_', byte(c))
			} else {
				output = append(output, byte(c))
			}
		}
		return strings.ToLower(string(output))
	}

	//Making a set of all columns in the models used
	modelColumnSet := map[string]bool{}
	addModelColumn := func(model interface{}) {
		m := reflect.TypeOf(model)
		for i := 0; i < m.NumField(); i++ {
			modelColumnSet[cleanKey(m.Field(i).Name)] = true
		}
	}
	//Add database models here
	addModelColumn(models.Metadata{})

	//Create sortOrder list
	sortOrderStringList := []string{}
	for _, sortOrder := range sortOrderList {
		sortOrderStringList = append(sortOrderStringList, sortOrder.Key+" "+sortOrder.Order)
	}

	//Create conditionString list
	conditionStringList := []string{}
	conditionValueList := []interface{}{}
	metaList := []metaDataQuery{}
	for _, condition := range conditionList {
		if modelColumnSet[condition.Key] {
			// if the key is a field in a model
			conditionStringList = append(conditionStringList, " "+condition.Key+" "+condition.Operator+" ?")
			if condition.Operator == "LIKE" {
				conditionValueList = append(conditionValueList, "%%"+condition.Value+"%%")
			} else if condition.Operator == "=" || condition.Operator == "<" || condition.Operator == ">" {
				conditionValueList = append(conditionValueList, condition.Value)
			} else {
				return nil, fmt.Errorf("invalid operator provided: %v ", condition.Operator)
			}
		} else {
			//if it is not a field in a model, then it must be present as key in fileMetadata
			var metaValue []interface{}
			metaString := "key = ? AND value " + condition.Operator + " ?"

			if condition.Operator == "LIKE" {
				metaValue = append(metaValue, condition.Key, "%%"+condition.Value+"%%")
			} else if condition.Operator == "=" {
				metaValue = append(metaValue, condition.Key, condition.Value)
			} else {
				return nil, fmt.Errorf("invalid operator provided: %v ", condition.Operator)
			}
			metaList = append(metaList, metaDataQuery{MetaString: metaString, MetaValue: metaValue})
		}
	}

	//Fetch file metadata
	metadataList := []models.Metadata{}
	queryChain := d.DB.Joins("LEFT JOIN file_metadata ON file_metadata.metadata_id = metadata.id").
		Where("clean_tombstone_set_at =  ?", 0)

	if len(conditionStringList) != 0 {
		queryChain = queryChain.Where(strings.Join(conditionStringList, " AND "), conditionValueList...)
	}
	for _, meta := range metaList {
		queryChain = queryChain.Where("(?) = metadata.id", d.DB.Select("metadata_id").
			Where("metadata_id = metadata.id").
			Where(meta.MetaString, meta.MetaValue...).
			Table("file_metadata"))
	}
	if len(sortOrderStringList) != 0 {
		queryChain = queryChain.Order(strings.Join(sortOrderStringList, ","))
	}
	queryChain.Distinct().
		Group("provided_name, provided_id").
		Having("created_at = max(created_at)").
		Preload("FileMetadata").
		Find(&metadataList)

	return metadataList, nil
}
