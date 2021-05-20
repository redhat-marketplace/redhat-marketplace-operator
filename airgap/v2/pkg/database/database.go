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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/go-logr/logr"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FileStore interface {
	SaveFile(finfo *v1.FileInfo, bs []byte) error
	DownloadFile(finfo *v1.FileID) (*models.Metadata, error)
	TombstoneFile(finfo *v1.FileID) error
	ListFileMetadata([]*Condition, []*SortOrder, bool) ([]models.Metadata, error)
	GetFileMetadata(finfo *v1.FileID) (*models.Metadata, error)
	CleanTombstones(before *timestamppb.Timestamp, PurgeAll bool) ([]*v1.FileID, error)
	UpdateFileMetadata(fid *v1.FileID, metadata map[string]string) error
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

	cb := sha256.Sum256(bs)
	c := hex.EncodeToString(cb[:])

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
		Checksum:        c,
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

	d.Log.Info("Saved file", "size", metadata.Size, "id", metadata.FileID, "checksum", c)
	return nil
}

// DownloadFile allows us to extract a file and it's metadata from the database provided it exists
func (d *Database) DownloadFile(finfo *v1.FileID) (*models.Metadata, error) {

	var meta models.Metadata

	fileid := strings.TrimSpace(finfo.GetId())
	filename := strings.TrimSpace(finfo.GetName())

	// Perform validations and query
	if len(fileid) != 0 {
		d.DB.Where("provided_id = ? AND deleted_at = ?", fileid, 0).
			Order("created_at desc").
			Preload(clause.Associations).
			First(&meta)
	} else if len(filename) != 0 {
		d.DB.Where("provided_name = ? AND deleted_at = ?", filename, 0).
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

// TombstoneFile marks file and its previous versions for deletion
func (d *Database) TombstoneFile(fid *v1.FileID) error {
	var meta models.Metadata
	now := time.Now()
	fileid := strings.TrimSpace(fid.GetId())
	filename := strings.TrimSpace(fid.GetName())

	if len(fileid) != 0 {
		d.DB.Model(&meta).
			Where("provided_id = ?", fileid).
			Update("clean_tombstone_set_at", now.Unix())
	} else if len(filename) != 0 {
		d.DB.Model(&meta).
			Where("provided_name = ?", filename).
			Update("clean_tombstone_set_at", now.Unix())
	} else {
		return fmt.Errorf("file id/name is blank")
	}
	d.Log.Info("File marked for delete", "id", fileid, "name", filename)
	return nil
}

// ListFileMetadata allow us to fetch list of files and its metadata from database
func (d *Database) ListFileMetadata(conditionList []*Condition, sortOrderList []*SortOrder, incDel bool) ([]models.Metadata, error) {
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
	queryChain := d.DB.Joins("LEFT JOIN file_metadata ON file_metadata.metadata_id = metadata.id")

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
	if incDel {
		queryChain.Distinct().
			Group("provided_name, provided_id").
			Having("created_at = max(created_at)").
			Having("deleted_at = 0").
			Preload("FileMetadata").
			Find(&metadataList)
	} else {
		queryChain.Where("clean_tombstone_set_at =  ?", 0).
			Distinct().
			Group("provided_name, provided_id").
			Having("created_at = max(created_at)").
			Having("deleted_at = 0").
			Preload("FileMetadata").
			Find(&metadataList)
	}
	return metadataList, nil
}

// GetFileMetadata allow us to fetch metadata of files from database
func (d *Database) GetFileMetadata(finfo *v1.FileID) (*models.Metadata, error) {

	var meta models.Metadata

	fileid := strings.TrimSpace(finfo.GetId())
	filename := strings.TrimSpace(finfo.GetName())

	// Perform validations and query
	if len(fileid) != 0 {
		d.DB.Where("provided_id = ?", fileid).
			Order("created_at desc").
			Preload("FileMetadata").
			First(&meta)
	} else if len(filename) != 0 {
		d.DB.Where("provided_name = ?", filename).
			Order("created_at desc").
			Preload("FileMetadata").
			First(&meta)
	} else {
		return nil, fmt.Errorf("file id/name is blank")
	}

	if reflect.DeepEqual(meta, models.Metadata{}) {
		return nil, fmt.Errorf("no file found for provided_name: %v / provided_id: %v", filename, fileid)
	}

	d.Log.Info("Retreived metadata of file", "id", meta.FileID)
	return &meta, nil
}

// CleanTombstones allows us to clear file content if purgeAll is false or delete file and it's database record if purgeAll is true
func (d *Database) CleanTombstones(before *timestamppb.Timestamp, purgeAll bool) ([]*v1.FileID, error) {

	metadataList := []models.Metadata{}
	if len(before.String()) != 0 {
		if purgeAll {
			d.DB.Select("file_id", "id", "provided_id", "provided_name").
				Where("clean_tombstone_set_at < (?) AND clean_tombstone_set_at > (?)", before.Seconds, 0).
				Find(&metadataList)
		} else {
			d.DB.Select("file_id", "id", "provided_id", "provided_name").
				Where("clean_tombstone_set_at < (?) AND clean_tombstone_set_at > (?)", before.Seconds, 0).
				Where("deleted_at = ?", 0).
				Find(&metadataList)
		}
	}

	var fileList []*v1.FileID
	var file_id []string
	var metadata_id []string
	for _, metadata := range metadataList {

		file_id = append(file_id, metadata.FileID)
		metadata_id = append(metadata_id, metadata.ID)

		if len(metadata.ProvidedId) != 0 {
			fid := v1.FileID{Data: &v1.FileID_Id{Id: metadata.ProvidedId}}
			fileList = append(fileList, &fid)
		} else if len(metadata.ProvidedName) != 0 {
			fid := v1.FileID{Data: &v1.FileID_Name{Name: metadata.ProvidedName}}
			fileList = append(fileList, &fid)
		}
	}

	if purgeAll {
		d.deleteFile(file_id, metadata_id)
	} else {
		d.clearFileContent(file_id)
	}

	return fileList, nil
}

// clearFileContent removes content of the files
func (d *Database) clearFileContent(fileList []string) {
	var content []byte
	for i := range fileList {
		d.DB.Model(&models.File{}).
			Where("id = ?", fileList[i]).
			Update("Content", content)

		// update deleted_at of the file
		d.DB.Model(&models.Metadata{}).
			Where("file_id = ?", fileList[i]).
			Update("deleted_at", time.Now().Unix())
	}
}

// deleteFile deletes all files and its metadata for given file_id and metadata_id
func (d *Database) deleteFile(file_id []string, metadata_id []string) {
	for i := range metadata_id {
		d.DB.Where("metadata_id = ?", metadata_id[i]).
			Delete(&models.FileMetadata{})
		d.DB.Where("id = ?", metadata_id[i]).
			Delete(&models.Metadata{})
	}
	for i := range file_id {
		d.DB.Where("id = ?", file_id[i]).
			Delete(&models.File{})
	}
}

// UpdateFileMetadata updates file info/metadata of provided FileID
func (d *Database) UpdateFileMetadata(fid *v1.FileID, metadata map[string]string) error {

	fileid := strings.TrimSpace(fid.GetId())
	filename := strings.TrimSpace(fid.GetName())

	if len(fileid) == 0 && len(filename) == 0 {
		return fmt.Errorf("file id/name is blank")
	}

	if len(metadata) == 0 {
		return fmt.Errorf("nil arguments received: metadata: %v ", metadata)
	} else {

		latestFileMetadata, err := d.GetFileMetadata(fid)
		if err != nil {
			return err
		}
		if reflect.DeepEqual(latestFileMetadata, models.Metadata{}) {
			return fmt.Errorf("no file found for provided_name: %v / provided_id: %v", filename, fileid)
		}

		matched := d.matchMetadata(latestFileMetadata.FileMetadata, metadata)
		if !matched {
			updatedMeta := d.createMetadataModels(*latestFileMetadata, metadata)
			d.updateFileMetadata(latestFileMetadata.ID, updatedMeta)

			var meta []models.Metadata
			if len(fileid) != 0 {
				d.DB.Select("*").
					Where("provided_id = ?", fileid).
					Not(models.Metadata{ID: latestFileMetadata.ID}).
					Find(&meta)

			} else if len(filename) != 0 {
				d.DB.Select("*").
					Where("provided_name = ?", filename).
					Not(models.Metadata{ID: latestFileMetadata.ID}).
					Find(&meta)
			}

			for _, m := range meta {
				updatedMeta := d.createMetadataModels(m, metadata)
				d.updateFileMetadata(m.ID, updatedMeta)
			}
		} else {
			return fmt.Errorf("connot update file metadata, as metadata of latest file and update request is same")
		}
	}

	return nil
}

// matchMetadata returns true if metadata in request and metadata of latest file matches else false
func (d *Database) matchMetadata(old []models.FileMetadata, updateTo map[string]string) bool {

	if len(updateTo) == 0 {
		return true
	} else if len(old) != len(updateTo) {
		return false
	}

	oldFms := make(map[string]string)
	for _, fm := range old {
		oldFms[fm.Key] = fm.Value
	}
	eq := reflect.DeepEqual(oldFms, updateTo)

	return eq
}

// createMetadataModels returns metadata object required for updating file metadata
func (d *Database) createMetadataModels(old models.Metadata, newMeta map[string]string) []models.FileMetadata {
	var fms []models.FileMetadata
	for k, v := range newMeta {
		fm := models.FileMetadata{
			MetadataID: old.ID,
			Key:        k,
			Value:      v,
		}
		fms = append(fms, fm)
	}

	return fms
}

// updateFileMetadata updates FileMetadata of provided id of file
func (d *Database) updateFileMetadata(id string, m []models.FileMetadata) {
	// deletes file metadata for given id
	d.DB.Where("metadata_id = ?", id).Delete(models.FileMetadata{})
	// insert new file metadata
	d.DB.Save(m)
}
