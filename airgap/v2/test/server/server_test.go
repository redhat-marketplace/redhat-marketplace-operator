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

package server_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"testing"

	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretreiver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	server "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/cmd/server/start"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener
var db database.Database
var dbName = "server.db"

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
		log.Fatalf("Error during creation of Database")
	}
	db.DB = gormDb
	db.Log = bs.Log

	//Create tables
	err = db.DB.AutoMigrate(&models.FileMetadata{}, &models.File{}, &models.Metadata{})
	if err != nil {
		log.Fatalf("Error during creation of Models: %v", err)
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
			log.Printf("Mock server started")
		}
	}()
}

func createClient() *grpc.ClientConn {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to dial bufnet: %v", err)
	}

	return conn
}

func shutdown(conn *grpc.ClientConn) {
	sqlDB, err := db.DB.DB()
	if err != nil {
		log.Fatalf("Error: Couldn't close Database: %v", err)
	}
	sqlDB.Close()
	conn.Close()
	os.Remove(dbName)
	lis.Close()
}

func TestFileSenderServer_UploadFile(t *testing.T) {
	//Initialize server
	runSetup()
	//Initialize client
	conn := createClient()
	client := filesender.NewFileSenderClient(conn)

	//Shutdown resources
	defer shutdown(conn)

	sampleData := make([]byte, 1024)
	tests := []struct {
		name    string
		info    *filesender.UploadFileRequest_Info
		data    *filesender.UploadFileRequest_ChunkData
		res     *filesender.UploadFileResponse
		errCode codes.Code
	}{
		{
			name: "invalid file upload request with nil file info/data",
			info: &filesender.UploadFileRequest_Info{
				Info: nil,
			},
			data: &filesender.UploadFileRequest_ChunkData{
				ChunkData: nil,
			},
			res:     &filesender.UploadFileResponse{},
			errCode: codes.Unknown,
		},
		{
			name: "valid file upload",
			info: &filesender.UploadFileRequest_Info{
				Info: &v1.FileInfo{
					FileId: &v1.FileID{
						Data: &v1.FileID_Name{
							Name: "test-file.zip",
						},
					},
					Size: 1024,
					Metadata: map[string]string{
						"key1": "value1",
						"key2": "value2",
						"key3": "value3",
					},
				},
			},
			data: &filesender.UploadFileRequest_ChunkData{
				ChunkData: sampleData,
			},
			res: &filesender.UploadFileResponse{
				Size: 1024,
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: "test-file.zip",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uploadClient, err := client.UploadFile(context.Background())
			if err != nil {
				t.Errorf("error while invoking grpc method upload file for test:%v with err: %v", tt.name, err)
			}

			err = uploadClient.Send(&filesender.UploadFileRequest{
				Data: tt.info,
			})
			if err != nil {
				t.Errorf("error while sending metadata for test:%v with err: %v", tt.name, err)
			}

			err = uploadClient.Send(&filesender.UploadFileRequest{
				Data: tt.data,
			})
			if err != nil {
				t.Errorf("error while uploading byte stream for test:%v with err: %v", tt.name, err)
			}

			res, err := uploadClient.CloseAndRecv()

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Errorf("mismatched error codes: expected %v, received: %v for test: %v", tt.errCode, er.Code(), tt.name)
					}
				}
			}

			if res != nil {
				if res.Size != tt.res.Size {
					t.Errorf("sent:%v and recieved:%v size doesn't match for test: %v", tt.res.Size, res.Size, tt.name)
				}
				if res.FileId.GetName() != tt.info.Info.FileId.GetName() {
					t.Errorf("name of uploaded file and downloaded file name does not match: %v != %v for test: %v", res.FileId.GetName(), tt.info.Info.FileId.GetName(), tt.name)
				}
			}
		})
	}
}

func TestFileRetreiverServer_DownloadFile(t *testing.T) {
	//Initialize server
	runSetup()
	//Initialize client
	conn := createClient()

	//Shutdown resources
	defer shutdown(conn)

	//Upload a sample file
	sampleData := make([]byte, 1024)
	client := filesender.NewFileSenderClient(conn)
	stream, err := client.UploadFile(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	err = stream.Send(&filesender.UploadFileRequest{
		Data: &filesender.UploadFileRequest_Info{
			Info: &v1.FileInfo{
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: "test-file.zip",
					},
				},
				Size: 1024,
				Metadata: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to upload file info: %v", err)
	}

	err = stream.Send(&filesender.UploadFileRequest{
		Data: &filesender.UploadFileRequest_ChunkData{
			ChunkData: sampleData,
		},
	})
	if err != nil {
		t.Fatalf("Failed to upload file data: %v", err)
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("Failed while closing stream: %v", err)
	}

	//Create a client for download
	downloadClient := fileretreiver.NewFileRetreiverClient(conn)

	tests := []struct {
		name    string
		dfr     *fileretreiver.DownloadFileRequest
		size    uint32
		errCode codes.Code
	}{
		{
			name: "download an existing file on the server",
			dfr: &fileretreiver.DownloadFileRequest{
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: "test-file.zip"},
				},
			},
			size:    1024,
			errCode: codes.OK,
		},
		{
			name: "invalid download request for file that doesn't exist on the server",
			dfr: &fileretreiver.DownloadFileRequest{
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: "dontexist.zip"},
				},
			},
			size:    0,
			errCode: codes.InvalidArgument,
		},
		{
			name: "invalid download request for file with only whitespaces for the name",
			dfr: &fileretreiver.DownloadFileRequest{
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: "   "},
				},
			},
			size:    0,
			errCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, err := downloadClient.DownloadFile(context.Background(), tt.dfr)

			if err != nil {
				t.Errorf("error while invoking grpc method download file for test:%v with err: %v", tt.name, err)
			}

			var bs bytes.Buffer
			for {
				data, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					if er, ok := status.FromError(err); ok {
						if er.Code() != tt.errCode {
							t.Errorf("mismatched error codes: expected %v, received: %v for test: %v", tt.errCode, er.Code(), tt.name)
						}
					}
					break
				}
				bs.Write(data.GetChunkData())
			}

			if bs.Len() != int(tt.size) {
				t.Errorf("sent:%v and recieved:%v size doesn't match for test: %v", bs.Len(), int(tt.size), tt.name)
			}
		})
	}
}
