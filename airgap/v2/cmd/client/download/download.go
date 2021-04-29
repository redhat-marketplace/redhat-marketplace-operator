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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretreiver"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/cmd/client/util"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

type DownloadConfig struct {
	fileName         string
	fileId           string
	outputDirectory  string
	fileListPath     string
	deleteOnDownload bool
	conn             *grpc.ClientConn
	client           fileretreiver.FileRetreiverClient
	log              logr.Logger
}

var (
	dc DownloadConfig
)

// DownloadCmd represents the download command
var DownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download files from the airgap service",
	Long:  `An external configuration file containing connection details are expected`,
	RunE: func(cmd *cobra.Command, args []string) error {

		initLog()
		// Initialize client
		err := dc.initializeDownloadClient()
		if err != nil {
			return err
		}
		defer dc.closeConnection()

		// If file list path is specified, perform batch download
		if len(strings.TrimSpace(dc.fileListPath)) != 0 {
			return dc.batchDownload()
		}

		// If file name/identifier is specified, download the single file
		return dc.downloadFile(dc.fileName, dc.fileId)
	},
}

func init() {
	DownloadCmd.Flags().StringVarP(&dc.fileName, "file-name", "n", "", "Name of the file to be downloaded")
	DownloadCmd.Flags().StringVarP(&dc.fileId, "file-id", "i", "", "Id of the file to be downloaded")
	DownloadCmd.Flags().StringVarP(&dc.outputDirectory, "output-directory", "o", "", "Path to download the file")
	DownloadCmd.Flags().StringVarP(&dc.fileListPath, "file-list-path", "f", "", "Fully qualified path to file containing list of names/identifiers")
	DownloadCmd.Flags().BoolVarP(&dc.deleteOnDownload, "delete-on-download", "D", false, "Mark file for deletion")
	DownloadCmd.MarkFlagRequired("output-directory")
}

//initLog initializes logger
func initLog() {
	var err error
	dc.log, err = util.InitLog()
	if err != nil {
		panic(err)
	}
}

// initializeDownloadClient initializes the file retriever client based on provided configuration parameters
func (dc *DownloadConfig) initializeDownloadClient() error {
	conn, err := util.InitClient()
	if err != nil {
		return err
	}
	dc.client = fileretreiver.NewFileRetreiverClient(conn)
	dc.conn = conn
	return nil
}

// closeConnection closes the grpc client connection
func (dc *DownloadConfig) closeConnection() {
	if dc != nil && dc.conn != nil {
		dc.conn.Close()
	}
}

// downloadFile downloads the file received from the grpc server to a specified directory
func (dc *DownloadConfig) downloadFile(fn string, fid string) error {
	fn = strings.TrimSpace(fn)
	fid = strings.TrimSpace(fid)
	var req *fileretreiver.DownloadFileRequest
	var name string
	var cs string

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
			MarkDelete: dc.deleteOnDownload,
		}
	} else {
		name = fid
		req = &fileretreiver.DownloadFileRequest{
			FileId: &v1.FileID{
				Data: &v1.FileID_Id{
					Id: fid},
			},
			MarkDelete: dc.deleteOnDownload,
		}
	}
	dc.log.Info("Attempting to download file", "name/id", name)

	resultStream, err := dc.client.DownloadFile(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to attempt download due to: %v", err)
	}

	var bs []byte
	for {
		file, err := resultStream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("error while reading stream: %v", err)
		}

		if len(strings.TrimSpace(file.GetInfo().GetChecksum())) != 0 {
			cs = file.GetInfo().GetChecksum()
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
	defer outFile.Close()

	if _, err := outFile.Write(bs); err != nil {
		return fmt.Errorf("error while writing data to the output file: %v", err)
	}

	dc.log.Info("File downloaded successfully!", "name/id", name)
	dc.log.Info("Checksum", "checksum", cs)
	return nil
}

// batchDownload will download all the files specified in the csv file generated by the list command
func (dc *DownloadConfig) batchDownload() error {
	fns, fids, err := parseCSV(dc.fileListPath)
	if err != nil {
		return err
	}

	for _, n := range fns {
		err = dc.downloadFile(n, "")
		if err != nil {
			dc.log.Error(err, "Error during download", "name", n)
		}
	}

	for _, id := range fids {
		err = dc.downloadFile("", id)
		if err != nil {
			dc.log.Error(err, "Error during download", "identifier", id)
		}
	}

	return nil
}

// parseCSV will read the csv file, extract identifiers/names and return them as slices
func parseCSV(fp string) (fns []string, fids []string, err error) {
	f, err := os.Open(fp)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	// Read csv headers
	chs, err := r.Read()
	if err != nil {
		return nil, nil, err
	}

	err = validateCSVHeaders(chs)
	if err != nil {
		return nil, nil, err
	}

	// Read records in the file
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}

		fid := strings.TrimSpace(record[0])
		if len(fid) != 0 {
			fids = append(fids, fid)
		}

		fn := strings.TrimSpace(record[1])
		if len(fn) != 0 {
			fns = append(fns, fn)
		}
	}

	return fns, fids, nil
}

// getExpectedCSVHeaders returns the minimum headers required to parse the csv file
func getExpectedCSVHeaders() []string {
	return []string{
		"File ID",
		"File Name",
	}
}

// validateCSVHeaders validates whether csv headers provided as argument match the order and size that's expected
func validateCSVHeaders(chs []string) error {
	echs := getExpectedCSVHeaders()
	if len(echs) > len(chs) {
		return fmt.Errorf("column count mismatch: expected: %v, received: %v", len(chs), len(echs))
	}

	for i, ecn := range echs {
		if ecn != chs[i] {
			return fmt.Errorf("column order mismatch: expected: %v, instead got: %v", ecn, chs[i])
		}
	}

	return nil
}
