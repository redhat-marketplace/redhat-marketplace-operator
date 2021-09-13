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
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretriever"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/cmd/client/util"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

type DownloadConfig struct {
	FileName         string
	FileId           string
	OutputDirectory  string
	FileListPath     string
	DeleteOnDownload bool
	conn             *grpc.ClientConn
	client           fileretriever.FileRetrieverClient
	log              logr.Logger
}

var (
	fileName         string
	fileId           string
	outputDirectory  string
	fileListPath     string
	deleteOnDownload bool
)

// DownloadCmd represents the download command
var DownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download files from the airgap service",
	Long:  `An external configuration file containing connection details are expected`,
	Example: `
    # Download file based using file name 
    client download --file-name file_name --output-directory /path/to/output/dir --config /path/to/config.yaml
    
    # Download file based using file id 
    client download --file-id file_id --output-directory /path/to/output/dir --config /path/to/config.yaml
    
    # Download files in batch using csv file 
    client download --file-list-path file_name --output-directory /path/to/output/dir --config /path/to/config.yaml
    
    # Mark file for deletion on download 
    client download -D --file-name file_name --output-directory /path/to/output/dir --config /path/to/config.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// create new download client
		dc, err := ProvideDownloadConfig(fileName, fileId, outputDirectory, fileListPath, deleteOnDownload)
		if err != nil {
			return err
		}
		defer dc.Close()

		// If file list path is specified, perform batch download
		if len(strings.TrimSpace(dc.FileListPath)) != 0 {
			return dc.BatchDownload()
		}

		// If file name/identifier is specified, download the single file
		return dc.DownloadFile(dc.FileName, dc.FileId)
	},
}

func init() {
	DownloadCmd.Flags().StringVarP(&fileName, "file-name", "n", "", "Name of the file to be downloaded")
	DownloadCmd.Flags().StringVarP(&fileId, "file-id", "i", "", "Id of the file to be downloaded")
	DownloadCmd.Flags().StringVarP(&outputDirectory, "output-directory", "o", "", "Path to download the file")
	DownloadCmd.Flags().StringVarP(&fileListPath, "file-list-path", "f", "", "Fully qualified path to file containing list of names/identifiers")
	DownloadCmd.Flags().BoolVarP(&deleteOnDownload, "delete-on-download", "D", false, "Mark file for deletion")
	DownloadCmd.MarkFlagRequired("output-directory")
}

func ProvideDownloadConfig(
	fileName string,
	fileId string,
	outputDirectory string,
	fileListPath string,
	deleteOnDownload bool,
) (*DownloadConfig, error) {
	log, err := util.InitLog()
	if err != nil {
		return nil, err
	}
	conn, err := util.InitClient()
	if err != nil {
		return nil, err
	}

	client := fileretriever.NewFileRetrieverClient(conn)

	return &DownloadConfig{
		FileName:         fileName,
		FileId:           fileId,
		OutputDirectory:  outputDirectory,
		FileListPath:     fileListPath,
		DeleteOnDownload: deleteOnDownload,
		conn:             conn,
		client:           client,
		log:              log,
	}, nil
}

// closeConnection closes the grpc client connection
func (dc *DownloadConfig) Close() {
	if dc != nil && dc.conn != nil {
		dc.conn.Close()
	}
}

// downloadFile downloads the file received from the grpc server to a specified directory
func (dc *DownloadConfig) DownloadFile(fn string, fid string) error {
	fn = strings.TrimSpace(fn)
	fid = strings.TrimSpace(fid)
	var req *fileretriever.DownloadFileRequest
	var name string
	var cs string

	// Validate input and prepare request
	if len(fn) == 0 && len(fid) == 0 {
		return fmt.Errorf("file id/name is blank")
	} else if len(fn) != 0 {
		name = fn
		req = &fileretriever.DownloadFileRequest{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: fn},
			},
			DeleteOnDownload: dc.DeleteOnDownload,
		}
	} else {
		name = fid
		req = &fileretriever.DownloadFileRequest{
			FileId: &v1.FileID{
				Data: &v1.FileID_Id{
					Id: fid},
			},
			DeleteOnDownload: dc.DeleteOnDownload,
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

	outFile, err := os.Create(dc.OutputDirectory + string(os.PathSeparator) + name)
	if err != nil {
		return fmt.Errorf("error while creating output file: %v", err)
	}
	defer outFile.Close()

	if _, err := outFile.Write(bs); err != nil {
		return fmt.Errorf("error while writing data to the output file: %v", err)
	}

	dc.log.Info("File downloaded successfully!", "name/id", name, "checksum", cs)
	return nil
}

// batchDownload will download all the files specified in the csv file generated by the list command
func (dc *DownloadConfig) BatchDownload() error {
	fns, fids, err := parseCSV(dc.FileListPath)
	if err != nil {
		return err
	}

	for _, n := range fns {
		err = dc.DownloadFile(n, "")
		if err != nil {
			dc.log.Error(err, "Error during download", "name", n)
		}
	}

	for _, id := range fids {
		err = dc.DownloadFile("", id)
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
