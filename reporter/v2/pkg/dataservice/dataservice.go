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
	"sync"
	"time"

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
	GetFile(ctx context.Context, id string) (*dataservicev1.FileInfo, error)
	Upload(context.Context, *dataservicev1.FileInfo, io.Reader) (id string, err error)
	UpdateMetadata(context.Context, *dataservicev1.FileInfo) (err error)
	DeleteFile(context.Context, *dataservicev1.FileInfo) error
}

type DataService struct {
	OutputPath string
	fileServer fileserver.FileServerClient

	opts  []grpc.CallOption
	mutex sync.RWMutex
}

var _ FileStorage = &DataService{}

func NewDataService(dataServiceConfig *DataServiceConfig, opts ...grpc.DialOption) (*DataService, error) {
	client, err := createDataServiceDownloadClient(context.Background(), dataServiceConfig, opts...)

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
	opts ...grpc.DialOption,
) (fileserver.FileServerClient, error) {
	logger.Info("airgap url", "url", dataServiceConfig.Address)

	conn, err := newGRPCConn(ctx,
		dataServiceConfig.Address,
		dataServiceConfig.DataServiceCert,
		dataServiceConfig.CipherSuites,
		dataServiceConfig.MinVersion,
		opts...)

	if err != nil {
		logger.Error(err, "failed to establish connection")
		return nil, err
	}

	return fileserver.NewFileServerClient(conn), nil
}

func (d *DataService) GetFile(ctx context.Context, id string) (*dataservicev1.FileInfo, error) {
	fileResp, err := d.fileServer.GetFile(ctx, &fileserver.GetFileRequest{
		IdLookup: &fileserver.GetFileRequest_Id{
			Id: id,
		},
	}, d.opts...)
	return fileResp.GetInfo(), err
}

func (d *DataService) ListFiles(ctx context.Context) ([]*dataservicev1.FileInfo, error) {
	fileList := []*dataservicev1.FileInfo{}
	pageToken := ""

	for {
		req := &fileserver.ListFilesRequest{
			PageSize:  100,
			PageToken: pageToken,
			Filter:    "",
		}

		response, err := d.fileServer.ListFiles(ctx, req, d.opts...)

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

func (d *DataService) UpdateMetadata(ctx context.Context, file *dataservicev1.FileInfo) error {
	_, err := d.fileServer.UpdateFileMetadata(ctx, &fileserver.UpdateFileMetadataRequest{
		Id:       file.Id,
		Metadata: file.Metadata,
	}, d.opts...)
	return err
}

func (d *DataService) DownloadFile(ctx context.Context, info *dataservicev1.FileInfo) (file string, err error) {
	if info == nil {
		err = errors.New("info provided is empty")
		return
	}

	req := &fileserver.DownloadFileRequest{
		Id: info.Id,
	}

	resultStream, err := d.fileServer.DownloadFile(ctx, req, d.opts...)
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

	// Always check whether os.File.Close returned an error and handle it appropriately.
	defer func() {
		err = errors.Append(err, f.Close())
	}()

	var resp *fileserver.DownloadFileResponse
	h := sha256.New()

	for {
		resp, err = resultStream.Recv()

		if err == io.EOF {
			// done reading
			err = nil
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
	}, d.opts...)

	if err != nil {
		logger.Info("failed to find, attempt upload", "err", err)
	}

	if resp != nil && resp.Info.Id != "" {
		info.Id = resp.Info.Id
	}

	var upload fileserver.FileServer_UploadFileClient
	upload, err = d.fileServer.UploadFile(ctx, d.opts...)

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
	}, d.opts...)

	return err
}

type DataServiceConfig struct {
	OutputPath       string   `json:"-"`
	Address          string   `json:"address"`
	DataServiceToken string   `json:"dataServiceToken"`
	DataServiceCert  []byte   `json:"dataServiceCert"`
	CipherSuites     []uint16 `json:"cipherSuites"`
	MinVersion       uint16   `json:"minVersion"`
}

func newGRPCConn(
	ctx context.Context,
	address string,
	caCert []byte,
	cipherSuites []uint16,
	minVersion uint16,
	opts ...grpc.DialOption,
) (*grpc.ClientConn, error) {

	/* creat tls */
	tlsConf, err := createTlsConfig(caCert, cipherSuites, minVersion)
	if err != nil {
		logger.Error(err, "failed to create creds")
		return nil, err
	}

	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConf)))

	opts = append(opts, grpc.WithBlock())

	context, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return grpc.DialContext(context, address, opts...)
}

func createTlsConfig(caCert []byte, cipherSuites []uint16, minVersion uint16) (*tls.Config, error) {
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
		RootCAs:      caCertPool,
		CipherSuites: cipherSuites,
		MinVersion:   minVersion,
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

// Set Call Options for NewStreams, usually for updated token & cert
func (r *DataService) SetCallOpts(opts ...grpc.CallOption) {
	r.mutex.Lock()
	r.opts = opts
	r.mutex.Unlock()
}

// Provide the Call Options for token auth
func ProvideGRPCCallOptions(
	dataServiceTokenFile string,
) ([]grpc.CallOption, error) {

	options := []grpc.CallOption{}

	var serviceAccountToken = ""
	if dataServiceTokenFile != "" {
		content, err := os.ReadFile(dataServiceTokenFile)
		if err != nil {
			return nil, err
		}
		serviceAccountToken = string(content)
	}

	if serviceAccountToken != "" {

		/* create oauth2 token  */
		oauth2Token := &oauth2.Token{
			AccessToken: serviceAccountToken,
		}

		perRPC := oauth.NewOauthAccess(oauth2Token)

		options = append(options, grpc.PerRPCCredentials(perRPC))
	}

	return options, nil
}
