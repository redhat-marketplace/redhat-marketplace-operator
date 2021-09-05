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
package list

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	logger "log"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/zapr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretreiver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/cmd/client/util"
	server "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/internal/server"
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
	bs := server.Server{}

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
	mockSenderServer := bs
	mockRetreiverServer := bs
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

func TestList(t *testing.T) {
	//Initialize the server
	runSetup()
	//Initialize connection
	conn := createClient()
	//Shutdown resources
	defer shutdown(conn)

	//Populate dataset for testing
	populateDataset(conn, t)
	listFilMetaDataCLient := fileretreiver.NewFileRetreiverClient(conn)
	od, _ := os.Getwd()
	tests := []struct {
		name   string
		lc     *ListConfig
		res    []string
		errMsg string
	}{
		{
			name: "fetch one of the files based on metadata and store to csv",
			lc: &ListConfig{
				Filter:    []string{"size GREATER_THAN 100", "type CONTAINS report"},
				Sort:      []string{},
				OutputDir: od,
				OutputCSV: true,
				FileName:  "files.csv",
				conn:      conn,
				client:    listFilMetaDataCLient,
			},
			res: []string{"marketplace_report.zip", "reports.zip"},
		},
		{
			name: "fetch one of the files based on metadata",
			lc: &ListConfig{
				Filter: []string{"size GREATER_THAN 100", "type CONTAINS report"},
				Sort:   []string{},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
		},
		{
			name: "all files are returned when no conditions are specified",
			lc: &ListConfig{
				Filter: []string{},
				Sort:   []string{},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
		},
		{
			name: "fetch latest file based on name",
			lc: &ListConfig{
				Filter: []string{"provided_name EQUAL reports.zip", "size GREATER_THAN 100", "version EQUAL 1"},
				Sort:   []string{"provided_name ASC"},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
		},
		{
			name: "fetch file for quoted metadata key value",
			lc: &ListConfig{
				Filter: []string{"'description    ' CONTAINS 'with builtin'"},
				Sort:   []string{},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
		},
		{
			name: "fetch file marked for deletion",
			lc: &ListConfig{
				Filter:              []string{"provided_name CONTAINS 'delete'"},
				Sort:                []string{},
				IncludeDeletedFiles: true,
				conn:                conn,
				client:              listFilMetaDataCLient,
			},
		},
		{
			name: "invalid Filter arguments",
			lc: &ListConfig{
				Filter: []string{"size GREATER_THAN 10 0"},
				Sort:   []string{},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
			errMsg: "'size GREATER_THAN 10 0' : invalid number of arguments provided for filter operation, Required 3 | Provided 4",
		},
		{
			name: "invalid filter operation",
			lc: &ListConfig{
				Filter: []string{"size GREATERTHAN 100"},
				Sort:   []string{},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
			errMsg: "invalid filter operation used",
		},
		{
			name: "invalid date format",
			lc: &ListConfig{
				Filter: []string{"created_at GREATER_THAN 21-4-13"},
				Sort:   []string{},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
			errMsg: "cannot parse",
		},
		{
			name: "invalid sort arguments",
			lc: &ListConfig{
				Filter: []string{},
				Sort:   []string{"asd"},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
			errMsg: "invalid number of arguments provided for sort operation, Required 2 | Provided 1",
		},
		{
			name: "invalid sort operation",
			lc: &ListConfig{
				Filter: []string{},
				Sort:   []string{"size ASCENDING"},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
			errMsg: "invalid sort operation used",
		},
		{
			name: "invalid filter operation using empty key/value",
			lc: &ListConfig{
				Filter: []string{"'    ' EQUAL '   ' "},
				Sort:   []string{},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
			errMsg: "invalid number of arguments provided for filter operation, Required 3 | Provided 1",
		},
		{
			name: "invalid filter operation using empty filter arguments list",
			lc: &ListConfig{
				Filter: []string{" "},
				Sort:   []string{},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
			errMsg: "invalid number of arguments provided for filter operation, Required 3 | Provided 0",
		},
		{
			name: "invalid sort operation using empty sort key/operation",
			lc: &ListConfig{
				Filter: []string{},
				Sort:   []string{" ' ' ASC"},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
			errMsg: "invalid number of arguments provided for sort operation, Required 2 | Provided 1",
		},
		{
			name: "invalid sort operation using empty sort arguments list",
			lc: &ListConfig{
				Filter: []string{},
				Sort:   []string{""},
				conn:   conn,
				client: listFilMetaDataCLient,
			},
			errMsg: "invalid number of arguments provided for sort operation, Required 2 | Provided 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.lc.log, _ = util.InitLog()
			err := tt.lc.listFileMetadata()

			if err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message: %v, instead got: %v", tt.errMsg, err.Error())
				}
			} else if len(tt.errMsg) > 0 {
				t.Errorf("Expected error: %v was never received!", tt.errMsg)
			}

			if tt.lc.OutputCSV {
				fp := tt.lc.OutputDir + string(os.PathSeparator) + tt.lc.FileName
				f, err := os.Open(fp)
				if err != nil {
					t.Errorf("Error opening file: %v", err)
				}
				r := csv.NewReader(f)
				// Read column names
				_, err = r.Read()
				if err != nil {
					t.Errorf("Error reading file: %v", err)
				}

				i := 0
				for {
					record, err := r.Read()
					if err == io.EOF {
						break
					}
					if tt.res[i] != strings.TrimSpace(record[1]) {
						t.Errorf("Expected name: %v, instead got: %v", tt.res[i], record[1])
					}
					i++
				}

				f.Close()
				err = os.Remove(fp)
				if err != nil {
					t.Errorf("Error removing file %v", err)
				}
			}
		})
	}
}

// populateDataset uploads files to the mock server
func populateDataset(conn *grpc.ClientConn, t *testing.T) {
	deleteFID := &v1.FileID{
		Data: &v1.FileID_Name{
			Name: "delete.txt",
		}}
	files := []v1.FileInfo{
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "reports.zip",
				},
			},
			Size:            1000,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version": "1",
				"type":    "report",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "reports.zip",
				},
			},
			Size:            2000,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version": "2",
				"type":    "report",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "marketplace_report.zip",
				},
			},
			Size:            200,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version": "2",
				"type":    "marketplace_report",
			},
		},
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: "dosbox",
				},
			},
			Size:            1500,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version":     "4.3",
				"description": "Emulator with builtin DOS for running DOS Games",
			},
		},
		{
			FileId:      deleteFID,
			Size:        1500,
			Compression: false,
			Metadata: map[string]string{
				"description": "file marked for deletion",
			},
		},
	}

	uploadClient := filesender.NewFileSenderClient(conn)

	// Upload files to mock server
	for i := range files {
		clientStream, err := uploadClient.UploadFile(context.Background())
		if err != nil {
			t.Fatalf("Error: During call of client.UploadFile: %v", err)
		}

		//Upload metadata
		err = clientStream.Send(&filesender.UploadFileRequest{
			Data: &filesender.UploadFileRequest_Info{
				Info: &files[i],
			},
		})

		if err != nil {
			t.Fatalf("Error: during sending metadata: %v", err)
		}

		//Upload chunk data
		bs := make([]byte, files[i].GetSize())
		request := filesender.UploadFileRequest{
			Data: &filesender.UploadFileRequest_ChunkData{
				ChunkData: bs,
			},
		}
		clientStream.Send(&request)

		res, err := clientStream.CloseAndRecv()
		if err != nil {
			t.Fatalf("Error: during stream close and recieve: %v", err)
		}
		t.Logf("Received response: %v", res)
		time.Sleep(1 * time.Second)
	}

	// Mark File for deletion
	req := &fileretreiver.DownloadFileRequest{
		FileId:           deleteFID,
		DeleteOnDownload: true,
	}
	dc := fileretreiver.NewFileRetreiverClient(conn)
	_, err := dc.DownloadFile(context.Background(), req)
	if err != nil {
		t.Fatalf("Error: during delete on download request : %v", err)
	}
}
