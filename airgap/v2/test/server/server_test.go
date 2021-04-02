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
	"bufio"
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
	lis.Close() //Closing This shuts down server too.
}

//A negative test for uploading nil values
func TestUploadFileEmptyStream(t *testing.T) {
	t.Log("Setting up test environment")
	runSetup()
	conn := createClient()
	client := filesender.NewFileSenderClient(conn)

	//Shutting down environment
	defer shutdown(conn)

	//client-stream to upload the file
	uploadClient, err := client.UploadFile(context.Background())
	if err != nil {
		t.Fatalf("error: During call of client.UploadFile: %v", err)
	}

	//Opening file
	filename := "file.gz"
	file, err := os.Open(filename)
	if err != nil {
		t.Fatalf("file couldn't be opened: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("error: couldn't close file after read: %v", err)
		}
	}()

	if err != nil {
		t.Fatalf("error: file Stats couldn't be obtained %v", err)
	}

	//Upload initial message & metadata
	err = uploadClient.Send(&filesender.UploadFileRequest{
		Data: &filesender.UploadFileRequest_Info{
			Info: nil,
		},
	})

	if err != nil && err != io.EOF {
		t.Fatalf("error: during sending of metadata: %v", err)
	}

	t.Log("finished uploading metadata")

	//Chunking and uploading empty stream
	chunkSize := 32
	buffReader := bufio.NewReader(file)
	buffer := make([]byte, chunkSize)
	for {
		_, err := buffReader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				t.Fatalf("Error: During sending of chunked data: %v", err)
			}
			break
		}
		//Sending request
		request := filesender.UploadFileRequest{
			Data: &filesender.UploadFileRequest_ChunkData{
				ChunkData: nil, //Request includes empty contents
			},
		}
		uploadClient.Send(&request)
	}

	t.Log("finished uploading file contents")

	//Closes stream
	_, err = uploadClient.CloseAndRecv()
	if err == nil {
		t.Error("error: uploading nil content is prohitbited", err)
	} else {
		t.Log("Attempt has been revoked successfully")
	}
}

//A positive test for uploading file
func TestUploadFile(t *testing.T) {
	t.Log("Setting up test environment")
	runSetup()
	conn := createClient()
	client := filesender.NewFileSenderClient(conn)

	//Shutting down environment
	defer shutdown(conn)

	//client-stream to upload the file
	uploadClient, err := client.UploadFile(context.Background())
	if err != nil {
		t.Fatalf("error: During call of client.UploadFile: %v", err)
	}

	//Opening file
	filename := "file.gz"
	file, err := os.Open(filename)
	if err != nil {
		t.Fatalf("file couldn't be opened: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("error: couldn't close file after read: %v", err)
		}
	}()

	//Getting file metadata
	metadata, err := file.Stat()
	if err != nil {
		t.Fatalf("error: file Stats couldn't be obtained %v", err)
	}

	//Upload initial message & metadata
	err = uploadClient.Send(&filesender.UploadFileRequest{
		Data: &filesender.UploadFileRequest_Info{
			Info: &v1.FileInfo{
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: metadata.Name(),
					},
				},
				Size: uint32(metadata.Size()),
				Metadata: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
		},
	})

	if err != nil && err != io.EOF {
		t.Fatalf("error: during sending of metadata: %v", err)
	}

	t.Log("finished uploading metadata")

	//Chunking
	chunkSize := 32
	buffReader := bufio.NewReader(file)
	buffer := make([]byte, chunkSize)
	for {
		n, err := buffReader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				t.Fatalf("Error: During sending of chunked data: %v", err)
			}
			break
		}
		//Sending request
		request := filesender.UploadFileRequest{
			Data: &filesender.UploadFileRequest_ChunkData{
				ChunkData: buffer[0:n],
			},
		}
		uploadClient.Send(&request)
	}

	t.Log("finished uploading file contents")

	//Closes stream
	res, err := uploadClient.CloseAndRecv()
	if err != nil {
		t.Fatalf("error: during stream close and recieve: %v", err)
	}

	t.Logf("recieved server response and closed stream")

	if res.Size != uint32(metadata.Size()) {
		t.Error(fmt.Sprintf("sent and recieved data size doesn't match: %v Bytes != %v Bytes", uint32(metadata.Size()), res.Size))
	} else {
		t.Log(fmt.Sprintf("verified size of sent and received files successfully:  %v Bytes == %v Bytes", uint32(metadata.Size()), res.Size))
	}

	if res.FileId.GetName() != metadata.Name() {
		t.Error(fmt.Sprintf("Name of sent file and recieved name does not match: %v != %v", metadata.Name(), res.FileId.GetName()))
	} else {
		t.Log(fmt.Sprintf("verified sent and received file names successfully: %v == %v", metadata.Name(), res.FileId.GetName()))
	}

}

//A positive test for Downloading File
func TestDownloadFile(t *testing.T) {
	t.Log("Setting up test environment")
	runSetup()
	conn := createClient()
	senderClient := filesender.NewFileSenderClient(conn)
	retreiverClient := fileretreiver.NewFileRetreiverClient(conn)

	//Shutting down environment
	defer shutdown(conn)

	//Opening file
	filename := "file.gz"
	file, err := os.Open(filename)
	if err != nil {
		t.Fatalf("file couldn't be opened: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("error: couldn't close file after read: %v", err)
		}
	}()

	//client-stream to upload the file
	uploadClient, err := senderClient.UploadFile(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	//Getting file metadata
	metadata, err := file.Stat()
	if err != nil {
		t.Fatalf("error: file Stats couldn't be obtained %v", err)
	}

	//Upload initial message & metadata
	err = uploadClient.Send(&filesender.UploadFileRequest{
		Data: &filesender.UploadFileRequest_Info{
			Info: &v1.FileInfo{
				FileId: &v1.FileID{
					Data: &v1.FileID_Name{
						Name: metadata.Name(),
					},
				},
				Size: uint32(metadata.Size()),
				Metadata: map[string]string{
					"key1": "value1",
				},
			},
		},
	})

	if err != nil && err != io.EOF {
		t.Fatalf("error: during sending of metadata: %v", err)
	}

	t.Log("finished uploading metadata")

	//Chunking
	chunkSize := 32
	buffReader := bufio.NewReader(file)
	buffer := make([]byte, chunkSize)
	for {
		n, err := buffReader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				t.Fatalf("Error: During sending of chunked data: %v", err)
			}
			break
		}
		//Sending request
		request := filesender.UploadFileRequest{
			Data: &filesender.UploadFileRequest_ChunkData{
				ChunkData: buffer[0:n],
			},
		}
		uploadClient.Send(&request)
	}

	t.Log("finished uploading file contents")

	//Closes stream
	_, err = uploadClient.CloseAndRecv()
	if err != nil {
		t.Fatalf("error: during stream close and receive: %v", err)
	}

	// Download file client request
	f_name := "file.gz"
	d_req := &fileretreiver.DownloadFileRequest{
		FileId: &v1.FileID{
			Data: &v1.FileID_Name{
				Name: f_name},
		},
	}
	downloadClient, err := retreiverClient.DownloadFile(context.Background(), d_req)
	if err != nil {
		log.Fatalf("failed to download: %v", err)
	}
	var bs bytes.Buffer
	for {
		res, err := downloadClient.Recv()
		if err == io.EOF {
			break
		}
		if err == nil {
			t.Log("File Chunk Received Successfully")
		} else {
			t.Errorf("Error receiving response: %v ", err)
			break
		}
		bs.Write(res.GetChunkData())
	}

	if metadata.Size() == int64(bs.Len()) {
		t.Log(fmt.Sprintf("Verified size of downloaded and existing files successfully:  %v Bytes == %v Bytes", uint32(metadata.Size()), uint32(metadata.Size())))
	} else {
		t.Errorf("Expected: %v, instead received: %v", metadata.Size(), bs.Len())
	}
}

//A negative test for Downloading File
func TestDownloadFileEmptyName(t *testing.T) {
	t.Log("Setting up test environment")
	runSetup()
	conn := createClient()
	client := fileretreiver.NewFileRetreiverClient(conn)

	//Shutting down environment
	defer shutdown(conn)

	// Download request with empty filename
	t.Log("Attempting to download file with whitespaces as name")
	d_req := &fileretreiver.DownloadFileRequest{
		FileId: &v1.FileID{
			Data: &v1.FileID_Name{
				Name: ""}, // Empty file name
		},
	}

	downloadClient, err := client.DownloadFile(context.Background(), d_req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, err = downloadClient.Recv()
	if err == io.EOF {
		t.Fatalf("Attempt has not been revoked successfully")
	} else if err != nil {
		t.Logf("Attempt has been revoked successfully: %v", err)
	} else {
		t.Error("Error: Cannot download file without name/id")
	}

	// Download non-existent file client request
	t.Log("Attempting to download non-existing file")
	d_req = &fileretreiver.DownloadFileRequest{
		FileId: &v1.FileID{
			Data: &v1.FileID_Name{
				Name: "file.zip",
			},
		},
	}

	downloadClient, err = client.DownloadFile(context.Background(), d_req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, err = downloadClient.Recv()
	if err == io.EOF {
		t.Fatalf("Attempt has not been revoked successfully")
	} else if err != nil {
		t.Logf("Attempt has been revoked successfully: %v", err)
	} else {
		t.Error("Error: Cannot download file without name/id")
	}

	t.Logf("Recieved server response and closed stream")
}
