package uploaders

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"gorm.io/gorm/logger"
)

type DataServiceConfig struct {
	Address          string `json:"address"`
	DataServiceToken string `json:"dataServiceToken"`
	DataServiceCert  []byte `json:"dataServiceCert"`
}

func provideDataServiceConfig(deployedNamespace string, dataServiceTokenFile string, dataServiceCertFile string) (*DataServiceConfig, error) {
	cert, err := ioutil.ReadFile(dataServiceCertFile)
	if err != nil {
		return nil, err
	}

	var serviceAccountToken = ""
	if dataServiceTokenFile != "" {
		content, err := ioutil.ReadFile(dataServiceTokenFile)
		if err != nil {
			return nil, err
		}
		serviceAccountToken = string(content)
	}

	var dataServiceDNS = fmt.Sprintf("%s.%s.svc:8004", utils.DATA_SERVICE_NAME, deployedNamespace)

	return &DataServiceConfig{
		Address:          dataServiceDNS,
		DataServiceToken: serviceAccountToken,
		DataServiceCert:  cert,
	}, nil
}

func NewDataServiceUploader(ctx context.Context, dataServiceConfig *DataServiceConfig) (Uploader, error) {
	uploadClient, err := createDataServiceUploadClient(ctx, dataServiceConfig)
	if err != nil {
		return nil, err
	}

	return &DataServiceUploader{
		UploadClient:      uploadClient,
		DataServiceConfig: *dataServiceConfig,
	}, nil
}

func createDataServiceUploadClient(ctx context.Context, dataServiceConfig *DataServiceConfig) (filesender.FileSender_UploadFileClient, error) {
	logger.Info("airgap url", "url", dataServiceConfig.Address)

	conn, err := newGRPCConn(ctx, dataServiceConfig.Address, dataServiceConfig.DataServiceCert, dataServiceConfig.DataServiceToken)

	if err != nil {
		logger.Error(err, "failed to establish connection")
		return nil, err
	}

	client := filesender.NewFileSenderClient(conn)

	uploadClient, err := client.UploadFile(context.Background())
	if err != nil {
		logger.Error(err, "could not initialize uploadClient")
		return nil, err
	}

	return uploadClient, nil
}

type DataServiceUploader struct {
	Ctx context.Context
	DataServiceConfig
	UploadClient filesender.FileSender_UploadFileClient
}

func (d *DataServiceUploader) UploadFile(path string) error {
	m := map[string]string{
		"version":    "v1",
		"reportType": "rhm-metering",
	}

	logger.Info("starting chunk and upload", "file name", path)
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer func() error {
		if err := file.Close(); err != nil {
			return err
		}

		return nil
	}()

	metaData, err := file.Stat()
	if err != nil {
		logger.Error(err, "Failed to get metadata")
		return err
	}

	err = d.UploadClient.Send(&filesender.UploadFileRequest{
		Data: &filesender.UploadFileRequest_Info{
			Info: &v1.FileInfo{
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: metaData.Name(),
					},
				},
				Size:     uint32(metaData.Size()),
				Metadata: m,
			},
		},
	})

	if err != nil {
		logger.Error(err, "Failed to create metadata UploadFile request")
		return err
	}

	chunkSize := 3
	buffReader := bufio.NewReader(file)
	buffer := make([]byte, chunkSize)
	for {
		n, err := buffReader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				logger.Error(err, "Error reading file")
			}
			break
		}

		request := filesender.UploadFileRequest{
			Data: &filesender.UploadFileRequest_ChunkData{
				ChunkData: buffer[0:n],
			},
		}
		err = d.UploadClient.Send(&request)
		if err != nil {
			logger.Error(err, "Failed to create UploadFile request")
			return err
		}
	}

	res, err := d.UploadClient.CloseAndRecv()
	if err != nil {
		if err == io.EOF {
			logger.Info("Stream EOF")
			return nil
		}

		logger.Error(err, "Error getting response")
		return err
	}

	logger.Info("airgap upload response", "response", res)

	return nil

}
