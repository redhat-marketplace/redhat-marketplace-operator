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

package download

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretreiver"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type DownloadConfig struct {
	fileName        string
	fileId          string
	outputDirectory string
	conn            *grpc.ClientConn
	client          fileretreiver.FileRetreiverClient
}

var (
	dc  DownloadConfig
	log logr.Logger
)

// DownloadCmd represents the download command
var DownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download files from the airgap service",
	Long:  `An external configuration file containing connection details are expected`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize client
		err := dc.initializeDownloadClient()
		if err != nil {
			return err
		}
		defer dc.closeConnection()

		// Download file and return error if any
		return dc.downloadFile()
	},
}

func init() {
	initLog()
	DownloadCmd.Flags().StringVarP(&dc.fileName, "file-name", "n", "", "Name of the file to be downloaded")
	DownloadCmd.Flags().StringVarP(&dc.fileId, "file-id", "i", "", "Id of the file to be downloaded")
	DownloadCmd.Flags().StringVarP(&dc.outputDirectory, "output-directory", "o", "", "Path to download the file ")
	DownloadCmd.MarkFlagRequired("output-directory")
}

func initLog() {
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize zapr, due to error: %v", err))
	}
	log = zapr.NewLogger(zapLog)
}

func (dc *DownloadConfig) initializeDownloadClient() error {
	// Fetch target address
	address := viper.GetString("address")
	if len(strings.TrimSpace(address)) == 0 {
		return fmt.Errorf("target address is blank/empty")
	}
	log.Info(fmt.Sprintf("Target address: %v", address))

	// Create connection
	insecure := viper.GetBool("insecure")
	var conn *grpc.ClientConn
	var err error

	if insecure {
		conn, err = grpc.Dial(address, grpc.WithInsecure())
	} else {
		cert := viper.GetString("certificate-path")
		creds, sslErr := credentials.NewClientTLSFromFile(cert, "")
		if sslErr != nil {
			return fmt.Errorf("ssl error: %v", sslErr)
		}
		opts := grpc.WithTransportCredentials(creds)
		conn, err = grpc.Dial(address, opts)
	}

	// Handle any connection errors
	if err != nil {
		return fmt.Errorf("connection error: %v", err)
	}

	dc.client = fileretreiver.NewFileRetreiverClient(conn)
	dc.conn = conn
	return nil
}

func (dc *DownloadConfig) closeConnection() {
	if dc != nil && dc.conn != nil {
		dc.conn.Close()
	}
}

func (dc *DownloadConfig) downloadFile() error {
	log.Info("Attempting to download file")

	fn := strings.TrimSpace(dc.fileName)
	fid := strings.TrimSpace(dc.fileId)
	var req *fileretreiver.DownloadFileRequest
	var name string

	// Validate input and prepare request
	if len(fn) == 0 && len(fid) == 0 {
		return fmt.Errorf("file id/name is blank")
	} else if len(fn) != 0 {
		name = fn
		req = &fileretreiver.DownloadFileRequest{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: fn},
			},
		}
	} else {
		name = fid
		req = &fileretreiver.DownloadFileRequest{
			FileId: &v1.FileID{
				Data: &v1.FileID_Id{
					Id: fid},
			},
		}
	}

	resultStream, err := dc.client.DownloadFile(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to attempt download due to: %v", err)
	}

	var bs []byte
	for {
		file, err := resultStream.Recv()
		if err == io.EOF {
			log.Info("Server stream end")
			break
		} else if err != nil {
			return fmt.Errorf("error while reading stream: %v", err)
		}

		data := file.GetChunkData()
		if bs == nil {
			bs = data
		} else {
			bs = append(bs, data...)
		}
	}

	outFile, err := os.Create(dc.outputDirectory + string(os.PathSeparator) + name)
	if err != nil {
		return fmt.Errorf("error while creating output file: %v", err)
	}

	outFile.Write(bs)
	outFile.Close()
	log.Info("File downloaded successfully!")
	return nil
}
