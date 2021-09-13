// Copyright 2020 IBM Corp.
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

package reporter

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretriever"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"

	"os"
	"path/filepath"
)

func (u *DataServiceDownloader) Name() string {
	return "data-service"
}

type Downloader interface {
	DownloadFile(path string) (string, error)
	ListFiles() ([]string, error)
}

type DataServiceDownloader struct {
	Ctx context.Context
	DataServiceConfig
	FileRetrieverClient fileretriever.FileRetrieverClient
}

func NewDataServiceDownloader(ctx context.Context, dataServiceConfig *DataServiceConfig) (Downloader, error) {
	client, err := createDataServiceDownloadClient(ctx, dataServiceConfig)

	if err != nil {
		return nil, err
	}

	return &DataServiceDownloader{
		FileRetrieverClient: client,
		DataServiceConfig:   *dataServiceConfig,
	}, nil
}

func createDataServiceDownloadClient(ctx context.Context, dataServiceConfig *DataServiceConfig) (fileretriever.FileRetrieverClient, error) {
	logger.Info("airgap url", "url", dataServiceConfig.Address)

	conn, err := newGRPCConn(ctx, dataServiceConfig.Address, dataServiceConfig.DataServiceCert, dataServiceConfig.DataServiceToken)

	if err != nil {
		logger.Error(err, "failed to establish connection")
		return nil, err
	}

	return fileretriever.NewFileRetrieverClient(conn), nil
}

func (d *DataServiceDownloader) ListFiles() ([]string, error) {
	var req *fileretriever.ListFileMetadataRequest

	fileList := []string{}

	req = &fileretriever.ListFileMetadataRequest{
		IncludeDeletedFiles: false,
	}

	resultStream, err := d.FileRetrieverClient.ListFileMetadata(context.Background(), req)
	if err != nil {
		return fileList, fmt.Errorf("error while opening stream: %v", err)
	}

	for {
		response, err := resultStream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return fileList, fmt.Errorf("error while reading stream: %v", err)
		}

		fileList = append(fileList, response.GetResults().FileId.GetName())
	}

	return fileList, nil

}

func (d *DataServiceDownloader) DownloadFile(path string) (string, error) {
	fn := strings.TrimSpace(path)
	var req *fileretriever.DownloadFileRequest

	// Validate input and prepare request
	if len(fn) == 0 {
		return "", fmt.Errorf("file id/name is blank")
	} else {
		req = &fileretriever.DownloadFileRequest{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: fn},
			},
			DeleteOnDownload: false,
		}
	}

	resultStream, err := d.FileRetrieverClient.DownloadFile(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to attempt download due to: %v", err)
	}

	newFilePath := filepath.Join(os.TempDir(), path)
	newFile, err := os.Create(newFilePath)
	if err != nil {
		panic(err)
	}
	defer newFile.Close()

	for {
		file, err := resultStream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", fmt.Errorf("error while reading stream: %v", err)
		}

		_, err = newFile.Write(file.GetChunkData())
		if err != nil {
			return "", fmt.Errorf("error while writing file: %v", err)
		}
	}

	return newFilePath, nil
}

func ProvideDownloader(
	ctx context.Context,
	cc ClientCommandRunner,
	log logr.Logger,
	reporterConfig *Config,
) (Downloader, error) {

	dataServiceConfig, err := provideDataServiceConfig(reporterConfig.DeployedNamespace, reporterConfig.DataServiceTokenFile, reporterConfig.DataServiceCertFile)
	if err != nil {
		return nil, err
	}

	return NewDataServiceDownloader(ctx, dataServiceConfig)

}
