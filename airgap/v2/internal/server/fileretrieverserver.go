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

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretriever"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FileRetrieverServer struct {
	*Server
	fileretriever.UnimplementedFileRetrieverServer
}

const chunkSize = 1024

type downloadFileResponseStream interface {
	Send(*fileretriever.DownloadFileResponse) error
}

func (frs *FileRetrieverServer) DownloadFile(dfr *fileretriever.DownloadFileRequest, stream fileretriever.FileRetriever_DownloadFileServer) error {
	return frs.downloadFile(dfr, stream)
}

// DownloadFile fetches the file from database, provided the file specified in the request exists
func (frs *FileRetrieverServer) downloadFile(dfr *fileretriever.DownloadFileRequest, stream downloadFileResponseStream) error {
	//Fetch file info from DB
	metadata, err := frs.FileStore.DownloadFile(dfr.GetFileId())
	if err != nil {
		return status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Failed to fetch file from database due to: %v", err),
		)
	}

	var fid v1.FileID
	if len(metadata.ProvidedId) != 0 {
		fid = v1.FileID{Data: &v1.FileID_Id{Id: metadata.ProvidedId}}
	} else if len(metadata.ProvidedName) != 0 {
		fid = v1.FileID{Data: &v1.FileID_Name{Name: metadata.ProvidedName}}
	}

	file_ := metadata.File
	fms := make(map[string]string)

	for _, fm := range metadata.FileMetadata {
		fms[fm.Key] = fm.Value
	}

	// File information response
	res := &fileretriever.DownloadFileResponse{
		Data: &fileretriever.DownloadFileResponse_Info{
			Info: &v1.FileInfo{
				FileId:           &fid,
				Size:             metadata.Size,
				CreatedAt:        ConvertTimestamp(metadata.CreatedAt),
				DeletedTombstone: ConvertTimestamp(metadata.DeletedAt),
				Compression:      metadata.Compression,
				CompressionType:  metadata.CompressionType,
				Checksum:         metadata.Checksum,
				Metadata:         fms,
			},
		},
	}
	frs.Log.Info("Response:", "file information", res)
	// Send file information
	err = stream.Send(res)
	if err != nil {
		return status.Errorf(
			codes.Unknown,
			fmt.Sprintf("Error while sending response: %v", err),
		)
	}

	buf := file_.Content
	var chunk []byte

	for len(buf) != 0 {
		if len(buf) >= chunkSize {
			chunk, buf = buf[:chunkSize], buf[chunkSize:]
		} else {
			chunk, buf = buf[:], buf[len(buf):]
		}

		// File chunk response
		res := &fileretriever.DownloadFileResponse{
			Data: &fileretriever.DownloadFileResponse_ChunkData{
				ChunkData: chunk,
			},
		}
		// Send file chunks
		err = stream.Send(res)
		if err != nil {
			return status.Errorf(
				codes.Unknown,
				fmt.Sprintf("Error while sending response %v ", err),
			)
		}
	}
	if dfr.GetDeleteOnDownload() {
		err = frs.FileStore.TombstoneFile(dfr.GetFileId())
		if err != nil {
			return status.Errorf(
				codes.InvalidArgument,
				fmt.Sprintf("Failed to mark file for deletion due to: %v", err),
			)
		}
	}
	return nil
}

type listFileMetadataResponseStream interface {
	Send(*fileretriever.ListFileMetadataResponse) error
}

func (frs *FileRetrieverServer) ListFileMetadata(lis *fileretriever.ListFileMetadataRequest, stream fileretriever.FileRetriever_ListFileMetadataServer) error {
	return frs.listFileMetadata(lis, stream)
}

// ListFileMetadata fetches list of files from database based on filters and sort conditions provided
func (frs *FileRetrieverServer) listFileMetadata(lis *fileretriever.ListFileMetadataRequest, stream listFileMetadataResponseStream) error {

	sortOrders := lis.GetSortBy()
	filters := lis.GetFilterBy()
	sortOrderList := []*database.SortOrder{}
	conditionList := []*database.Condition{}

	for _, sortOrder := range sortOrders {
		key := strings.TrimSpace(sortOrder.GetKey())
		order := strings.TrimSpace(sortOrder.GetSortOrder().String())
		if len(key) == 0 {
			return status.Errorf(
				codes.InvalidArgument,
				"Cannot pass empty key for sort operation",
			)
		}
		sortOrderList = append(sortOrderList, &database.SortOrder{Key: key, Order: order})
	}
	//parse and convert raw filter operators to database friendly operators
	for _, filter := range filters {
		key := strings.TrimSpace(filter.GetKey())
		val := strings.TrimSpace(filter.GetValue())
		rawOperator := strings.TrimSpace(filter.GetOperator().String())

		if len(key) == 0 || len(val) == 0 {
			return status.Errorf(
				codes.InvalidArgument,
				"Cannot pass empty key/value for filter operation",
			)
		}

		var operator string
		switch rawOperator {
		case "EQUAL":
			operator = "="
		case "LESS_THAN":
			operator = "<"
		case "GREATER_THAN":
			operator = ">"
		case "CONTAINS":
			operator = "LIKE"
		default:
			return status.Errorf(
				codes.InvalidArgument,
				fmt.Sprintf("Invalid operator used for filter operation: %v ", rawOperator),
			)
		}
		conditionList = append(conditionList, &database.Condition{Key: key, Value: val, Operator: operator})
	}

	//Fetching metadata
	metadataList, err := frs.FileStore.ListFileMetadata(conditionList, sortOrderList, lis.IncludeDeletedFiles)
	if err != nil {
		return status.Errorf(
			codes.Unknown,
			fmt.Sprintf("Error Fetching File List %v ", err),
		)
	}

	//Preparing and sending responses
	for _, metadata := range metadataList {

		//Preparing file metadata
		fileMetadata := map[string]string{}
		for _, fmd := range metadata.FileMetadata {
			fileMetadata[fmd.Key] = fmd.Value
		}

		//Either use FileID_Id or FileID_Name
		var fileId v1.FileID
		if metadata.ProvidedId != "" {
			fileId = v1.FileID{
				Data: &v1.FileID_Id{
					Id: metadata.ProvidedId,
				},
			}
		} else {
			fileId = v1.FileID{
				Data: &v1.FileID_Name{
					Name: metadata.ProvidedName,
				},
			}
		}

		//Build response
		response := &fileretriever.ListFileMetadataResponse{
			Results: &v1.FileInfo{
				FileId:           &fileId,
				Size:             metadata.Size,
				Metadata:         fileMetadata,
				CreatedAt:        ConvertTimestamp(metadata.CreatedAt),
				DeletedTombstone: ConvertTimestamp(metadata.DeletedAt),
				Compression:      metadata.Compression,
				CompressionType:  metadata.CompressionType,
				Checksum:         metadata.Checksum,
			},
		}
		//Send response
		stream.Send(response)
	}

	return nil
}

// GetFileMetadata fetches the file metadata from database, provided the file specified in the request exists
func (frs *FileRetrieverServer) GetFileMetadata(ctx context.Context, in *fileretriever.GetFileMetadataRequest) (*fileretriever.GetFileMetadataResponse, error) {
	//Fetch file info from DB
	metadata, err := frs.FileStore.GetFileMetadata(in.GetFileId())
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Failed to fetch file metadata from database due to: %v", err),
		)
	}

	var fid v1.FileID
	if len(metadata.ProvidedId) != 0 {
		fid = v1.FileID{Data: &v1.FileID_Id{Id: metadata.ProvidedId}}
	} else if len(metadata.ProvidedName) != 0 {
		fid = v1.FileID{Data: &v1.FileID_Name{Name: metadata.ProvidedName}}
	}

	fms := make(map[string]string)
	for _, fm := range metadata.FileMetadata {
		fms[fm.Key] = fm.Value
	}

	// File information response
	res := &fileretriever.GetFileMetadataResponse{
		Info: &v1.FileInfo{
			FileId:           &fid,
			Size:             metadata.Size,
			CreatedAt:        ConvertTimestamp(metadata.CreatedAt),
			DeletedTombstone: ConvertTimestamp(metadata.DeletedAt),
			Compression:      metadata.Compression,
			CompressionType:  metadata.CompressionType,
			Metadata:         fms,
		},
	}
	frs.Log.Info("File Info Response", "res", res)
	// Send file information
	return res, nil
}
