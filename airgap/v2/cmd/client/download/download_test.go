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
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretriever"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/cmd/client/util"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/internal/clienttest"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener
var db database.Database
var dbName = "client.db"
var conn *grpc.ClientConn
var downloadClient fileretriever.FileRetrieverClient

func TestDownload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer os.Remove(dbName)

	lis = bufconn.Listen(bufSize)
	defer lis.Close()

	clienttest.SetupServer(t, ctx, lis, &db, dbName)

	//Initialize connection
	//Populate data on the mock server
	conn = clienttest.NewGRPCClient(ctx, lis)
	defer conn.Close()

	downloadClient = fileretriever.NewFileRetrieverClient(conn)

	fns := populateDataset(t, ctx)

	t.Run("downloadFile", testDownloadFile)

	//Create csv files required for batch download
	fps := createCSVFiles(fns, t)

	//Delete csv files
	defer deleteFiles(fps)

	t.Run("downloadBatch", testBatchDownload(fns, fps))
}

func testDownloadFile(t *testing.T) {
	//Initialize the server
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
				OutputDirectory: od,
				FileName:        "reports.zip",
				conn:            conn,
				client:          downloadClient,
			},
			size:   1000,
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
			tt.dc.log, _ = util.InitLog()
			err := tt.dc.DownloadFile(tt.dc.FileName, tt.dc.FileId)
			if err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message: %v, instead got: %v", tt.errMsg, err.Error())
				}
			} else if len(tt.errMsg) > 0 {
				t.Errorf("Expected error: %v was never received!", tt.errMsg)
			} else {
				file, err := os.Open(tt.dc.FileName)
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
				os.Remove(tt.dc.FileName)
			}
		})
	}
}

func testBatchDownload(fns []string, fps []string) func(t *testing.T) {
	return func(t *testing.T) {
		od, _ := os.Getwd()
		tests := []struct {
			name   string
			dc     *DownloadConfig
			errMsg string
		}{
			{
				name: "valid batch download request",
				dc: &DownloadConfig{
					OutputDirectory: od,
					FileListPath:    fps[0],
					conn:            conn,
					client:          downloadClient,
				},
			},
			{
				name: "invalid request with csv having insufficient headers",
				dc: &DownloadConfig{
					OutputDirectory: od,
					FileListPath:    fps[1],
					conn:            conn,
					client:          downloadClient,
				},
				errMsg: "column count mismatch",
			},
			{
				name: "invalid request with csv having headers in the wrong order",
				dc: &DownloadConfig{
					OutputDirectory: od,
					FileListPath:    fps[2],
					conn:            conn,
					client:          downloadClient,
				},
				errMsg: "column order mismatch",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.dc.log, _ = util.InitLog()
				err := tt.dc.BatchDownload()
				if err != nil {
					if !strings.Contains(err.Error(), tt.errMsg) {
						t.Errorf("Expected error message: %v, instead got: %v", tt.errMsg, err.Error())
					}
				} else if len(tt.errMsg) > 0 {
					t.Errorf("Expected error: %v was never received!", tt.errMsg)
				} else {
					fns, _, _ = parseCSV(tt.dc.FileListPath)
					defer deleteFiles(fns)

					for _, fn := range fns {
						finfo, fErr := os.Stat(fn)
						if os.IsNotExist(fErr) {
							t.Fatalf("File from csv wasn't downloaded: %v", fn)
						}
						t.Logf("File downloaded successfully: %v", finfo.Name())
					}
				}
			})
		}
	}
}

// createCSVFiles creates csv files required for the batch download, returns fully qualified paths of created files
func createCSVFiles(fns []string, t *testing.T) (fps []string) {
	od, _ := os.Getwd()
	od = od + string(os.PathSeparator)

	files := []struct {
		headers []string
		fns     []string
		ofn     string
	}{
		{
			headers: getExpectedCSVHeaders(),
			fns:     fns,
			ofn:     "valid_file.csv",
		},
		{
			headers: getExpectedCSVHeaders()[:1],
			fns:     nil,
			ofn:     "invalid_header_count.csv",
		},
		{
			headers: reverseSlice(getExpectedCSVHeaders()),
			fns:     nil,
			ofn:     "invalid_header_order.csv",
		},
	}

	for _, f := range files {
		fps = append(fps, od+f.ofn)
		file, err := os.Create(od + f.ofn)
		if err != nil {
			t.Fatalf("Failed to create csv file due to %v", err)
		}
		w := csv.NewWriter(file)
		//Write headers
		err = w.Write(f.headers)
		if err != nil {
			t.Fatalf("Failed to write to csv file due to %v", err)
		}

		for _, fn := range f.fns {
			w.Write([]string{"", fn})
		}
		w.Flush()
		file.Close()
	}

	return fps
}

// reverseSlice will reverse a given slice
func reverseSlice(s []string) []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// deleteFiles deletes files at specified file paths
func deleteFiles(fps []string) error {
	for _, fp := range fps {
		err := os.Remove(fp)
		if err != nil {
			return err
		}
	}
	return nil
}

// populateDataset uploads files to the mock server and returns the file names if upload is successful
func populateDataset(t *testing.T, ctx context.Context) []string {
	fns := []string{"reports.zip", "marketplace_report.zip"}
	files := []v1.FileInfo{
		{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: fns[0],
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
					Name: fns[1],
				},
			},
			Size:            300,
			Compression:     true,
			CompressionType: "gzip",
			Metadata: map[string]string{
				"version": "1",
				"type":    "marketplace_report",
			},
		},
	}

	uploadClient := filesender.NewFileSenderClient(conn)

	// Upload files to mock server
	for i := range files {
		clientStream, err := uploadClient.UploadFile(ctx)
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

		err = clientStream.Send(&request)
		if err != nil {
			t.Fatalf("Error: during send: %v", err)
		}

		res, err := clientStream.CloseAndRecv()
		if err != nil {
			t.Fatalf("Error: during stream close and recieve: %v", err)
		}
		t.Logf("Received response: %v", res)
	}

	return fns
}
