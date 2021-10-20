package modelsv2

import (
	"fmt"
	"strconv"

	"github.com/golang/protobuf/ptypes"
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

func StoredFileFromProto(finfo dataservicev1.FileInfo) (*StoredFile, error) {
	metadata := []StoredFileMetadata{}

	for key, value := range finfo.Metadata {
		metadata = append(metadata, StoredFileMetadata{
			Key:   key,
			Value: value,
		})
	}

	return &StoredFile{
		Name:         finfo.Name,
		Source:       finfo.Source,
		SourceType:   finfo.SourceType,
		FileMetadata: metadata,
	}, nil
}

func StoredFileToProto(file StoredFile) (fileInfo *dataservicev1.FileInfo, err error) {
	var createdAt, updatedAt, deletedAt *timestamppb.Timestamp

	if createdAt, err = ptypes.TimestampProto(file.CreatedAt); err != nil {
		return
	}
	if updatedAt, err = ptypes.TimestampProto(file.UpdatedAt); err != nil {
		return
	}
	if file.DeletedAt.Valid {
		if deletedAt, err = ptypes.TimestampProto(file.DeletedAt.Time); err != nil {
			return
		}
	}

	metadata := map[string]string{}

	for _, md := range file.FileMetadata {
		metadata[md.Key] = md.Value
	}

	fileInfo = &dataservicev1.FileInfo{
		Id:         fmt.Sprintf("%d", file.ID),
		Name:       file.Name,
		Source:     file.Source,
		SourceType: file.SourceType,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		DeletedAt:  deletedAt,
		Metadata:   metadata,
	}

	return
}
