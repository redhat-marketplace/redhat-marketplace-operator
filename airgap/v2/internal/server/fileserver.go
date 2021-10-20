package server

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"emperror.dev/errors"
	dataservicev1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1/fileserver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	modelsv2 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FileServer struct {
	*Server
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
				fmt.Sprintf("Error while processing stream, details: %v", err),
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
	file, err := modelsv2.StoredFileFromProto(*finfo)

	file.File.Content = bs
	file.File.MimeType = finfo.MimeType

	// Attempt to save file in database
	id, err := fs.FileStore.Save(file)
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
		Id:   fmt.Sprintf("%i", id),
		Size: uint32(file.File.Size),
	}

	return stream.SendAndClose(res)
}

func (fs *FileServer) ListFiles(ctx context.Context, req *fileserver.ListFilesRequest) (*fileserver.ListFilesResponse, error) {
	var (
		page int
		err  error
	)

	if req.PageToken != "" {
		page, err = strconv.Atoi(req.PageToken)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "pageToken must be a valid page")
		}
	} else {
		page = 0
	}

	opts := []database.ListOption{
		database.Paginate(page, int(req.PageSize)),
	}

	responseFiles := []*dataservicev1.FileInfo{}

	files, err := fs.FileStore.List(opts...)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list files")
	}

	errs := []error{}

	for _, file := range files {
		protoFile, err := modelsv2.StoredFileToProto(file)

		if err != nil {
			errs = append(errs, err)
			continue
		}

		responseFiles = append(responseFiles, protoFile)
	}

	err = errors.Combine(errs...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert files", "err", err)
	}

	return &fileserver.ListFilesResponse{
		Files: responseFiles,
	}, nil
}

func (fs *FileServer) GetFile(ctx context.Context, req *fileserver.GetFileRequest) (*fileserver.GetFileResponse, error) {
	file, err := fs.FileStore.Get(req.Id)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get file", "id", req.Id, "err", err)
	}

	info, err := modelsv2.StoredFileToProto(*file)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert file", "id", req.Id, "err", err)
	}

	return &fileserver.GetFileResponse{
		Info: info,
	}, nil
}

func (fs *FileServer) UpdateFileMetadata(ctx context.Context, req *fileserver.UpdateFileMetadataRequest) (*fileserver.UpdateFileMetadataResponse, error) {
	file, err := fs.FileStore.Get(req.Id)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get file", "id", req.Id, "err", err)
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

	_, err = fs.FileStore.Save(file)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save file", "id", req.Id, "err", err)
	}

	protoFile, err := modelsv2.StoredFileToProto(*file)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert file", "id", req.Id, "err", err)
	}

	return &fileserver.UpdateFileMetadataResponse{
		File: protoFile,
	}, nil
}

func (fs *FileServer) DownloadFile(req *fileserver.DownloadFileRequest, stream fileserver.FileServer_DownloadFileServer) error {
	file, err := fs.FileStore.Download(req.Id)
	if err != nil {
		return status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Failed to fetch file from database due to: %v", err),
		)
	}

	protoFile, err := modelsv2.StoredFileToProto(*file)

	if err != nil {
		return status.Errorf(codes.Internal, "failed to convert file to proto", "err", err)
	}

	// File information response
	res := &fileserver.DownloadFileResponse{
		Data: &fileserver.DownloadFileResponse_Info{
			Info: protoFile,
		},
	}

	fs.Log.V(4).Info("Response file information", "response", res)
	// Send file information
	err = stream.Send(res)
	if err != nil {
		return status.Errorf(
			codes.Unknown,
			fmt.Sprintf("Error while sending response: %v", err),
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
			Data: &fileserver.DownloadFileResponse_ChunkData{
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

	return nil
}

func (fs *FileServer) DeleteFile(ctx context.Context, req *fileserver.DeleteFileRequest) (*fileserver.DeleteFileResponse, error) {
	err := fs.FileStore.Delete(req.Id)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete file", "id", req.Id, "err", err)
	}

	return &fileserver.DeleteFileResponse{Id: req.Id}, nil
}

func (fs *FileServer) CleanTombstones(context.Context, *fileserver.CleanTombstonesRequest) (*fileserver.CleanTombstonesResponse, error) {
	rowsAffects, err := fs.FileStore.CleanTombstones()

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to clean tombstones", "err", err)
	}

	return &fileserver.CleanTombstonesResponse{TombstonesCleaned: int32(rowsAffects)}, nil
}
