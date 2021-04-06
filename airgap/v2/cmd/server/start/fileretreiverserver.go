package server

import (
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretreiver"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const chunkSize = 1024

type FileRetreiverServer struct {
	fileretreiver.UnimplementedFileRetreiverServer
	B BaseServer
}

func (frs *FileRetreiverServer) DownloadFile(dfr *fileretreiver.DownloadFileRequest, stream fileretreiver.FileRetreiver_DownloadFileServer) error {

	//Fetch file info from DB
	metadata, err := frs.B.FileStore.DownloadFile(dfr.GetFileId())
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

	created_at, _ := ptypes.TimestampProto(time.Unix(metadata.CreatedAt, 0))
	deleted_at, _ := ptypes.TimestampProto(time.Unix(metadata.DeletedAt, 0))

	// File information response
	res := &fileretreiver.DownloadFileResponse{
		Data: &fileretreiver.DownloadFileResponse_Info{
			Info: &v1.FileInfo{
				FileId:           &fid,
				Size:             metadata.Size,
				CreatedAt:        created_at,
				DeletedTombstone: deleted_at,
				UpdatedAt:        created_at,
				Compression:      metadata.Compression,
				CompressionType:  metadata.CompressionType,
				Metadata:         fms,
			},
		},
	}
	frs.B.Log.Info(fmt.Sprintf("File Info Response: %v ", res))
	// Send file information
	err = stream.Send(res)
	if err != nil {
		return status.Errorf(
			codes.Unknown,
			fmt.Sprintf("Error sending Response: %v", err),
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
		res := &fileretreiver.DownloadFileResponse{
			Data: &fileretreiver.DownloadFileResponse_ChunkData{
				ChunkData: chunk,
			},
		}
		// Send file chunks
		err = stream.Send(res)
		if err != nil {
			return status.Errorf(
				codes.Unknown,
				fmt.Sprintf(" Error sending Response %v ", err),
			)
		}
	}
	return nil
}
