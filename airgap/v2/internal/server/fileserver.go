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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"emperror.dev/errors"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	dataservicev1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1/fileserver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	modelsv2 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/status"
)

type FileServer struct {
	*Server
	Health *health.Server
	fileserver.UnimplementedFileServerServer
}

const (
	ErrFileChecksumIncorrect = errors.Sentinel("calculated checksum does not match")
	ErrFileInfoMissing       = errors.Sentinel("no file info provided")
	ErrFileContentMissing    = errors.Sentinel("no file data provided")
)

func (fs *FileServer) UploadFile(stream fileserver.FileServer_UploadFileServer) error {
	var bs []byte
	var finfo *dataservicev1.FileInfo

	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			fs.Log.Error(err, "Oops, something went wrong!")
			return status.Errorf(
				codes.Unknown,
				"Error while processing stream, details: %v",
				err,
			)
		}

		if b := req.GetChunkData(); b != nil {
			if bs == nil {
				bs = b
			} else {
				bs = append(bs, b...)
			}
		}

		if req.GetInfo() != nil {
			finfo = req.GetInfo()
		}
	}

	if finfo == nil {
		fs.Log.Error(ErrFileInfoMissing, "file info is nil")
		return status.Errorf(
			codes.FailedPrecondition,
			ErrFileInfoMissing.Error(),
		)
	}

	if len(bs) == 0 {
		fs.Log.Error(ErrFileContentMissing, "file content is missing")
		return status.Errorf(
			codes.FailedPrecondition,
			ErrFileContentMissing.Error(),
		)
	}

	fs.Log.V(2).Info("Stream end", "total bytes received", len(bs))
	file, err := modelsv2.StoredFileFromProto(finfo)

	if err != nil {
		return err
	}

	file.File.Content = bs
	file.File.MimeType = finfo.MimeType

	// Attempt to save file in database
	id, err := fs.FileStore.Save(stream.Context(), file)
	if err != nil {
		return status.Errorf(
			codes.Unknown,
			fmt.Sprintf("Failed to save file in database: %v", err),
		)
	}

	if file.File.Checksum != finfo.Checksum {
		err := errors.WithDetails(ErrFileChecksumIncorrect, "checksumProvided", finfo.Checksum, "checksumCalculated", file.File.Checksum)
		fs.Log.Error(err, "checksum failure")

		return status.Errorf(
			codes.DataLoss,
			"Err: %s Details: %v+",
			err.Error(),
			errors.GetDetails(err),
		)
	}

	// Prepare response on save and close stream
	res := &fileserver.UploadFileResponse{
		Id:   id,
		Size: uint32(file.File.Size),
	}

	return stream.SendAndClose(res)
}

func (fs *FileServer) ListFiles(ctx context.Context, req *fileserver.ListFilesRequest) (*fileserver.ListFilesResponse, error) {
	pageSize := 100
	opts := []database.ListOption{}

	if req.PageSize != 0 {
		pageSize = int(req.PageSize)
	}

	opts = append(opts, database.Paginate(req.PageToken, pageSize))

	if req.IncludeDeleted {
		opts = append(opts, database.ShowDeleted())
	}

	responseFiles := []*dataservicev1.FileInfo{}

	files, pageToken, err := fs.FileStore.List(ctx, opts...)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list files")
	}

	errs := []error{}

	for i := range files {
		protoFile, err := modelsv2.StoredFileToProto(files[i])

		if err != nil {
			errs = append(errs, err)
			continue
		}

		responseFiles = append(responseFiles, protoFile)
	}

	err = errors.Combine(errs...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert files. Error: %s", err)
	}

	return &fileserver.ListFilesResponse{
		Files:         responseFiles,
		NextPageToken: pageToken,
		PageSize:      int32(pageSize),
	}, nil
}

func (fs *FileServer) GetFile(ctx context.Context, req *fileserver.GetFileRequest) (*fileserver.GetFileResponse, error) {
	var (
		file *modelsv2.StoredFile
		err  error
	)

	if req.GetId() != "" {
		file, err = fs.FileStore.Get(ctx, req.GetId())

		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get file %s=%s %s=%s", "id", req.GetId(), "err", err)
		}
	} else if req.GetKey() != nil {
		key := req.GetKey()
		file, err = fs.FileStore.GetByFileKey(ctx, &modelsv2.StoredFileKey{
			Name:       key.Name,
			Source:     key.Source,
			SourceType: key.SourceType,
		})

		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get file %s=%s %s=%s", "id", req.GetKey(), "err", err)
		}
	} else {
		return nil, status.Errorf(codes.Internal, "no key provided")
	}

	if file == nil {
		return nil, status.Errorf(codes.NotFound, "not found")
	}

	info, err := modelsv2.StoredFileToProto(*file)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert file %s=%d %s=%s", "id", file.ID, "err", err)
	}

	return &fileserver.GetFileResponse{
		Info: info,
	}, nil
}

func (fs *FileServer) UpdateFileMetadata(
	ctx context.Context,
	req *fileserver.UpdateFileMetadataRequest,
) (*fileserver.UpdateFileMetadataResponse, error) {
	file, err := fs.FileStore.Get(ctx, req.Id)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get file %s=%s %s=%s", "id", req.Id, "err", err)
	}

	metadata := req.Metadata

	for _, md := range file.FileMetadata {
		if _, ok := metadata[md.Key]; !ok {
			metadata[md.Key] = md.Value
		}
	}

	fileMetadata := []modelsv2.StoredFileMetadata{}

	for key, value := range metadata {
		fileMetadata = append(fileMetadata,
			modelsv2.StoredFileMetadata{
				Key:   key,
				Value: value,
			})
	}

	file.FileMetadata = fileMetadata

	_, err = fs.FileStore.Save(ctx, file)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save file %s=%s %s=%s", "id", req.Id, "err", err)
	}

	protoFile, err := modelsv2.StoredFileToProto(*file)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert file %s=%s %s=%s", "id", req.Id, "err", err)
	}

	return &fileserver.UpdateFileMetadataResponse{
		File: protoFile,
	}, nil
}

const chunkSize = 1024

func (fs *FileServer) DownloadFile(req *fileserver.DownloadFileRequest, stream fileserver.FileServer_DownloadFileServer) error {
	file, err := fs.FileStore.Download(stream.Context(), req.Id)
	if err != nil {
		return status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Failed to fetch file from database due to: %v", err),
		)
	}

	buf := file.File.Content
	var chunk []byte

	for len(buf) != 0 {
		if len(buf) >= chunkSize {
			chunk, buf = buf[:chunkSize], buf[chunkSize:]
		} else {
			chunk, buf = buf[:], buf[len(buf):]
		}

		// File chunk response
		res := &fileserver.DownloadFileResponse{
			ChunkData: chunk,
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

	return nil
}

func (fs *FileServer) DeleteFile(ctx context.Context, req *fileserver.DeleteFileRequest) (*fileserver.DeleteFileResponse, error) {
	err := fs.FileStore.Delete(ctx, req.Id, req.Permanent)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete file %s=%s %s=%s", "id", req.Id, "err", err)
	}

	return &fileserver.DeleteFileResponse{Id: req.Id}, nil
}

func (fs *FileServer) CleanTombstones(ctx context.Context, _ *fileserver.CleanTombstonesRequest) (*fileserver.CleanTombstonesResponse, error) {
	rowsAffects, err := fs.FileStore.CleanTombstones(ctx)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to clean tombstones %s=%s", "err", err)
	}

	return &fileserver.CleanTombstonesResponse{TombstonesCleaned: int32(rowsAffects)}, nil
}

func (fs *FileServer) RegisterHTTPRoutes(mux *runtime.ServeMux) error {
	return mux.HandlePath("POST", "/v1/file/{id}/download", fs.httpDownloadFile)
}

func (fs *FileServer) httpDownloadFile(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	id, ok := pathParams["id"]

	if !ok {
		http.Error(w, fmt.Sprintf("file id is not provided"), http.StatusBadRequest)
		return
	}

	data, err := fs.FileStore.Download(ctx, id)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if data == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Add("Content-Type", http.DetectContentType(data.File.Content))
	w.Header().Add("Digest", fmt.Sprintf("sha-256=%s", data.File.Checksum))
	io.Copy(w, bytes.NewReader(data.File.Content))
	return
}
