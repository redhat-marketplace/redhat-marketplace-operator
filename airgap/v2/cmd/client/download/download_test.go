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
	logger "log"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/go-logr/zapr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretreiver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	server "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/cmd/server/start"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener
var db database.Database
var dbName = "client.db"

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func runSetup() {
	//Initialize the mock connection and server
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	bs := server.BaseServer{}
	mockSenderServer := server.FileSenderServer{}
	mockRetreiverServer := server.FileRetreiverServer{}

	//Initialize logger
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize zapr, due to error: %v", err))
	}
	bs.Log = zapr.NewLogger(zapLog)

	//Create Sqlite Database
	gormDb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		logger.Fatalf("Error during creation of Database")
	}
	db.DB = gormDb
	db.Log = bs.Log

	//Create tables
	err = db.DB.AutoMigrate(&models.FileMetadata{}, &models.File{}, &models.Metadata{})
	if err != nil {
		logger.Fatalf("Error during creation of Models: %v", err)
	}

	bs.FileStore = &db
	mockSenderServer.B = bs
	mockRetreiverServer.B = bs
	filesender.RegisterFileSenderServer(s, &mockSenderServer)
	fileretreiver.RegisterFileRetreiverServer(s, &mockRetreiverServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			if err.Error() != "closed" { //When lis of type (*bufconn.Listener) is closed, server doesn't have to panic.
				panic(err)
			}
		} else {
			logger.Printf("Mock server started")
		}
	}()
}

func createClient() *grpc.ClientConn {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		logger.Fatalf("failed to dial bufnet: %v", err)
	}

	return conn
}

func shutdown(conn *grpc.ClientConn) {
	sqlDB, err := db.DB.DB()
	if err != nil {
		logger.Fatalf("Error: Couldn't close Database: %v", err)
	}
	sqlDB.Close()
	conn.Close()
	os.Remove(dbName)
	lis.Close()
}

func TestDownloadFile(t *testing.T) {
	//Initialize the server
	runSetup()

	//Initialize connection
	conn := createClient()

	//Shutdown resources
	defer shutdown(conn)

	//Upload a sample file
	uploadClient := filesender.NewFileSenderClient(conn)
	clientStream, err := uploadClient.UploadFile(context.Background())
	if err != nil {
		t.Fatalf("error: During call of client.UploadFile: %v", err)
	}

	// Upload metadata
	err = clientStream.Send(&filesender.UploadFileRequest{
		Data: &filesender.UploadFileRequest_Info{
			Info: &v1.FileInfo{
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: "test.zip",
					},
				},
				Size: 1024,
			},
		},
	})
	if err != nil {
		t.Fatalf("error: during sending of metadata: %v", err)
	}

	//Upload contents
	bs := make([]byte, 1024)
	request := filesender.UploadFileRequest{
		Data: &filesender.UploadFileRequest_ChunkData{
			ChunkData: bs,
		},
	}
	clientStream.Send(&request)

	res, err := clientStream.CloseAndRecv()
	if err != nil {
		t.Fatalf("error: during stream close and recieve: %v", err)
	}
	t.Log(fmt.Sprintf("Received response: %v", res))

	downloadClient := fileretreiver.NewFileRetreiverClient(conn)
	od, _ := os.Getwd()
	tests := []struct {
		name   string
		dc     *DownloadConfig
		size   uint32
		errMsg string
	}{
		{
			name: "downloading a file that exists on the server",
			dc: &DownloadConfig{
				outputDirectory: od,
				fileName:        "test.zip",
				conn:            conn,
				client:          downloadClient,
			},
			size:   1024,
			errMsg: "",
		},
		{
			name:   "invalid download request with no file name/id provided",
			dc:     &DownloadConfig{},
			errMsg: "file id/name is blank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dc.downloadFile()
			if err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message: %v, instead got: %v", tt.errMsg, err.Error())
				}
			} else if len(tt.errMsg) > 0 {
				t.Errorf("Expected error: %v was never received!", tt.errMsg)
			} else {
				file, err := os.Open(tt.dc.fileName)
				if err != nil {
					t.Errorf("file was not downloaded correctly for test: %v, error: %v", tt.name, err)
				}

				finfo, err := file.Stat()
				if err != nil {
					t.Errorf("path error for test: %v, error: %v", tt.name, err)
				}

				if finfo.Size() != int64(tt.size) {
					t.Errorf("file size doesn't match: expected %v, received: %v", tt.size, finfo.Size())
				}

				file.Close()
				os.Remove(tt.dc.fileName)
			}
		})
	}
}
