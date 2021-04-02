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

package client_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"testing"

	"github.com/go-logr/zapr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretreiver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/cmd/client/download"
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

func setupFileRetreiverServer() {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	bs := server.BaseServer{}
	mockRetreiverServer := server.FileRetreiverServer{}
	mockSenderServer := server.FileSenderServer{}

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
	mockRetreiverServer.B = bs
	mockSenderServer.B = bs
	filesender.RegisterFileSenderServer(s, &mockSenderServer)
	fileretreiver.RegisterFileRetreiverServer(s, &mockRetreiverServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			if err.Error() != "closed" {
				panic(err)
			}
		} else {
			log.Printf("Mock server started")
		}
	}()
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

func createFileRetreiverClient() (*grpc.ClientConn, fileretreiver.FileRetreiverClient) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to dial bufnet: %v", err)
	}
	return conn, fileretreiver.NewFileRetreiverClient(conn)
}

func TestDownloadFile(t *testing.T) {
	setupFileRetreiverServer()
	// Upload a file
	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to create upload client: %v", err)
	}
	defer conn.Close()

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

	if err != nil && err != io.EOF {
		t.Fatalf("error: during sending of metadata: %v", err)
	}

	// Upload contents
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

	// Download a file through client
	conn, client := createFileRetreiverClient()
	od, _ := os.Getwd()
	dc := &download.DownloadConfig{
		OutputDirectory: od,
		FileName:        "test.zip",
		Conn:            conn,
		Client:          client,
	}
	defer shutdown(conn)

	err = dc.DownloadFile()
	if err != nil {
		t.Fatalf("Failed to download file: %v", err)
	}

	file, err := os.Open("test.zip")
	if err != nil {
		t.Fatalf("File was not downloaded correctly!")
	}

	if file.Name() != "test.zip" {
		t.Fatalf("File saved with incorrect name!")
	}

	finfo, err := file.Stat()
	if err != nil {
		t.Fatalf("path error: %v", err)
	}

	if finfo.Size() != 1024 {
		t.Fatalf("File sizes don't match!")
	}

	file.Close()
	os.Remove("test.zip")
}

func TestDownloadFileInputValidation(t *testing.T) {
	// nil name and id
	dc := &download.DownloadConfig{}
	err := dc.DownloadFile()
	if err == nil {
		t.Fatal("Download allows nil values for id/name!")
	}
}
