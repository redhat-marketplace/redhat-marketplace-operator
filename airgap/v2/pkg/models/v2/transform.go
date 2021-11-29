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
	"fmt"
	"strconv"

	dataservicev1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ConvertStrToUint(id string) (idInt uint, err error) {
	var idInt64 uint64
	idInt64, err = strconv.ParseUint(id, 10, 0)

	if err != nil {
		return
	}

	idInt = uint(idInt64)
	return
}

func StoredFileFromProto(finfo *dataservicev1.FileInfo) (*StoredFile, error) {
	metadata := []StoredFileMetadata{}

	for key, value := range finfo.Metadata {
		metadata = append(metadata, StoredFileMetadata{
			Key:   key,
			Value: value,
		})
	}

	return &StoredFile{
		Name:       finfo.Name,
		Source:     finfo.Source,
		SourceType: finfo.SourceType,
		Metadata:   metadata,
	}, nil
}

func StoredFileToProto(file *StoredFile) (fileInfo *dataservicev1.FileInfo, err error) {
	var createdAt, updatedAt *timestamppb.Timestamp

	createdAt = timestamppb.New(file.CreatedAt)
	updatedAt = timestamppb.New(file.UpdatedAt)

	fileInfo = &dataservicev1.FileInfo{
		Id:         fmt.Sprintf("%d", file.ID),
		Name:       file.Name,
		Source:     file.Source,
		SourceType: file.SourceType,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		Checksum:   file.File.Checksum,
		Size:       uint32(file.File.Size),
		MimeType:   file.File.MimeType,
		Metadata:   map[string]string{},
	}

	for i := range file.Metadata {
		fileInfo.Metadata[file.Metadata[i].Key] = file.Metadata[i].Value
	}

	if !file.DeletedAt.Time.IsZero() {
		deletedAt := timestamppb.New(file.DeletedAt.Time)
		fileInfo.DeletedAt = deletedAt
	}

	return
}
