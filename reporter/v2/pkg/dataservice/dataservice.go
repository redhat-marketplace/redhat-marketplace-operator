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

package dataservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"emperror.dev/errors"
	dataservicev1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1/fileserver"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	logger = logf.Log.WithName("data_service")
)

type FileStorage interface {
	DownloadFile(context.Context, *dataservicev1.FileInfo) (file string, err error)
	ListFiles(context.Context) ([]*dataservicev1.FileInfo, error)
	Upload(context.Context, *dataservicev1.FileInfo, io.Reader) (id string, err error)
	DeleteFile(context.Context, *dataservicev1.FileInfo) error
}

type DataService struct {
	OutputPath string
	fileServer fileserver.FileServerClient
}

var _ FileStorage = &DataService{}

func NewDataService(dataServiceConfig *DataServiceConfig) (*DataService, error) {
	client, err := createDataServiceDownloadClient(context.Background(), dataServiceConfig)

	if err != nil {
		return nil, err
	}

	return &DataService{
		fileServer: client,
		OutputPath: dataServiceConfig.OutputPath,
	}, nil
}

func (u *DataService) Name() string {
	return "data-service"
}

func createDataServiceDownloadClient(
	ctx context.Context,
	dataServiceConfig *DataServiceConfig,
) (fileserver.FileServerClient, error) {
	logger.Info("airgap url", "url", dataServiceConfig.Address)

	conn, err := newGRPCConn(ctx, dataServiceConfig.Address, dataServiceConfig.DataServiceCert, dataServiceConfig.DataServiceToken)

	if err != nil {
		logger.Error(err, "failed to establish connection")
		return nil, err
	}

	return fileserver.NewFileServerClient(conn), nil
}

func (d *DataService) ListFiles(ctx context.Context) ([]*dataservicev1.FileInfo, error) {
	fileList := []*dataservicev1.FileInfo{}
	pageToken := ""

	for {
		req := &fileserver.ListFilesRequest{
			PageSize:  100,
			PageToken: pageToken,
		}

		response, err := d.fileServer.ListFiles(ctx, req)

		if err != nil {
			return fileList, fmt.Errorf("error while opening stream: %v", err)
		}

		fileList = append(fileList, response.GetFiles()...)

		if response.NextPageToken == "" {
			break
		}

		pageToken = response.NextPageToken
	}

	return fileList, nil

}

func (d *DataService) DownloadFile(ctx context.Context, info *dataservicev1.FileInfo) (file string, err error) {
	if info == nil {
		err = errors.New("info provided is empty")
		return
	}

	req := &fileserver.DownloadFileRequest{
		Id: info.Id,
	}

	resultStream, err := d.fileServer.DownloadFile(ctx, req)
	if err != nil {
		err = fmt.Errorf("failed to attempt download due to: %v", err)
		return
	}

	file = filepath.Join(d.OutputPath, info.Name)
	f, err := os.OpenFile(file, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)

	if err != nil {
		err = fmt.Errorf("failed to attempt download due to: %v", err)
		return
	}

	defer f.Close()

	var resp *fileserver.DownloadFileResponse
	h := sha256.New()

	for {
		resp, err = resultStream.Recv()

		if err == io.EOF {
			break
		}

		if err != nil {
			err = fmt.Errorf("error while reading stream: %v", err)
			return
		}

		if data := resp.GetChunkData(); data != nil {
			_, err = f.Write(data)

			if err != nil {
				err = errors.Wrap(err, "failed to write file")
				return
			}

			_, err = h.Write(data)

			if err != nil {
				err = errors.Wrap(err, "failed to write file checksum")
				return
			}
		}
	}

	checksum := fmt.Sprintf("%x", h.Sum(nil))

	err = f.Close()
	if err != nil {
		err = errors.Wrap(err, "failed to close the file")
		return
	}

	if info.Checksum != checksum {
		err = errors.NewWithDetails("file checksum doesn't match", "id", info.Id, "server", info.Checksum, "client", checksum)
		return
	}

	return
}

const chunkSize = 1024

func (d *DataService) Upload(ctx context.Context, info *dataservicev1.FileInfo, reader io.Reader) (id string, err error) {
	if info == nil {
		err = errors.New("info provided is empty")
		return
	}

	resp, err := d.fileServer.GetFile(ctx, &fileserver.GetFileRequest{
		IdLookup: &fileserver.GetFileRequest_Key{
			Key: &dataservicev1.FileKey{
				Name:       info.Name,
				Source:     info.Source,
				SourceType: info.SourceType,
			},
		},
	})

	if err != nil {
		logger.Info("failed to find, attempt upload", "err", err)
	}

	if resp != nil && resp.Info.Id != "" {
		info.Id = resp.Info.Id
	}

	var upload fileserver.FileServer_UploadFileClient
	upload, err = d.fileServer.UploadFile(ctx)

	if err != nil {
		logger.Error(err, "Failed to UploadFile request")
		return
	}

	err = upload.Send(&fileserver.UploadFileRequest{
		Data: &fileserver.UploadFileRequest_Info{
			Info: info,
		},
	})

	if err != nil {
		logger.Error(err, "Failed to UploadFile request")
		return
	}

	var n int
	buffer := make([]byte, chunkSize)

	for {
		n, err = reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}

			logger.Error(err, "failed to read file")
			return
		}

		request := fileserver.UploadFileRequest{
			Data: &fileserver.UploadFileRequest_ChunkData{
				ChunkData: buffer[0:n],
			},
		}

		err = upload.Send(&request)
		if err != nil {
			logger.Error(err, "Failed to create UploadFile request")
			return
		}
	}

	res, err := upload.CloseAndRecv()
	if err != nil {
		if err == io.EOF {
			logger.Info("Stream EOF")
			return
		}

		logger.Error(err, "Error getting response")
		return
	}

	id = res.Id

	logger.Info("airgap upload response", "response", res)
	return
}

func (d *DataService) DeleteFile(ctx context.Context, info *dataservicev1.FileInfo) error {
	if info == nil {
		return errors.New("info provided is empty")
	}

	_, err := d.fileServer.DeleteFile(ctx, &fileserver.DeleteFileRequest{
		Id: info.Id,
	})

	return err
}

type DataServiceConfig struct {
	OutputPath       string `json:"-"`
	Address          string `json:"address"`
	DataServiceToken string `json:"dataServiceToken"`
	DataServiceCert  []byte `json:"dataServiceCert"`
}

func newGRPCConn(
	ctx context.Context,
	address string,
	caCert []byte,
	token string,
) (*grpc.ClientConn, error) {

	options := []grpc.DialOption{}

	/* creat tls */
	tlsConf, err := createTlsConfig(caCert)
	if err != nil {
		logger.Error(err, "failed to create creds")
		return nil, err
	}

	if token != "" {
		options = append(options, grpc.WithTransportCredentials(credentials.NewTLS(tlsConf)))

		/* create oauth2 token  */
		oauth2Token := &oauth2.Token{
			AccessToken: token,
		}

		perRPC := oauth.NewOauthAccess(oauth2Token)

		options = append(options, grpc.WithPerRPCCredentials(perRPC))
	}

	options = append(options, grpc.WithBlock())

	return grpc.DialContext(ctx, address, options...)
}

func createTlsConfig(caCert []byte) (*tls.Config, error) {
	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get system cert pool")
	}

	ok := caCertPool.AppendCertsFromPEM(caCert)
	if !ok {
		err = errors.New("failed to append cert to cert pool")
		logger.Error(err, "cert pool error")
		return nil, err
	}

	return &tls.Config{
		RootCAs: caCertPool,
	}, nil
}

func (r *DataService) UploadFile(ctx context.Context, fileName string, reader io.Reader) (string, error) {
	info := &dataservicev1.FileInfo{
		Name:       filepath.Base(fileName),
		Source:     "redhat-marketplace",
		SourceType: "report",
		MimeType:   "application/gzip",
	}

	b, err := io.ReadAll(reader)

	if err != nil {
		return "", err
	}

	mV := ctx.Value("metadata")

	if mV != nil {
		metadata, ok := mV.(*MeterReportMetadata)

		if ok {
			if out, err := metadata.Map(); err == nil {
				info.Metadata = out
			}
		}
	}

	checksum := sha256.Sum256(b)
	info.Checksum = fmt.Sprintf("%x", checksum)

	return r.Upload(ctx, info, bytes.NewReader(b))
}
