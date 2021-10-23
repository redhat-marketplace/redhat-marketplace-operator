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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/olekukonko/tablewriter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretriever"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/cmd/client/util"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

type ListConfig struct {
	Filter              []string
	Sort                []string
	OutputDir           string
	OutputCSV           bool
	IncludeDeletedFiles bool
	FileName            string
	conn                *grpc.ClientConn
	client              fileretriever.FileRetrieverClient
	log                 logr.Logger
}

var (
	filter              []string
	sort                []string
	outputDir           string
	outputCSV           bool
	includeDeletedFiles bool
	fileName            = "files.csv"
)

var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "Fetch list of files",
	Long: `Fetch list of files

Filter flags used for filtering list to be fetched using pre-defined 
keys or custom key and sort flag used for sorting list based on sort key and sort order.

    Allowed filter operators: EQUAL, CONTAINS, LESS_THAN, GREATER_THAN
    Allowed sort operators: ASC, DESC
    -----------------------------------------------------------------------
    Pre-defined filter keys: 
    [provided_id] refers to the file identifier
    [provided_name] refers to the name of the file
    [size] refers to the size of file
    [created_at] refers to file creation date (expected format yyyy-mm-dd or RFC3339)
    [deleted_at] refers to the file deletion date  (expected format yyyy-mm-dd or RFC3339)
    -----------------------------------------------------------------------
    Pre-defined sort keys: 
    [provided_id] refers to the file identifier
    [provided_name] refers to the name of the file
    [size] refers to the size of file
    [created_at] refers to file creation date (expected format yyyy-mm-dd or RFC3339)
    [deleted_at] refers to the file deletion date  (expected format yyyy-mm-dd or RFC3339)
	`,
	Example: `
    # List all latest files.
    client list --config /path/to/config.yaml
	
    # List all files between specific dates.
    client list --filter "created_at GREATER_THAN 2020-12-25" --filter "created_at LESS_THAN 2021-03-30" --config /path/to/config.yaml
	
    # List files having specific metadata
    client list --filter "description CONTAINS 'operator file'" --config /path/to/config.yaml
	
    # List files uploaded after specific dates and sort it by file name in ascending order
    client list --filter "created_at GREATER_THAN 2021-03-20" --sort "provided_name ASC" --config /path/to/config.yaml
	
    # Sort list by size in ascending order
    client list -sort "size ASC" --config /path/to/config.yaml
    
    # List files marked for deleteion
    client list --include-deleted-files --config /path/to/config.yaml
	
    # Save list to csv file
    client list  --output-dir=/path/to/dir --config /path/to/config.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*60*time.Second)
		defer cancel()

		// create a list client
		lc, err := ProvideListConfig(ctx, filter, sort, outputDir, outputCSV, includeDeletedFiles, fileName)
		if err != nil {
			return err
		}

		if cmd.Flag("output-dir").Changed {
			lc.OutputCSV = true
		}

		defer lc.closeConnection()
		return lc.listFileMetadata(ctx)
	},
}

func init() {
	ListCmd.Flags().StringSliceVarP(&filter, "filter", "f", []string{}, "Filter file list based on pre-defined or custom keys")
	ListCmd.Flags().StringSliceVarP(&sort, "sort", "s", []string{}, "Sort file list based key and sort operation used")
	ListCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Path to save list")
	ListCmd.Flags().BoolVarP(&includeDeletedFiles, "include-deleted-files", "a", false, "List all files along with files marked for deleteion")
}

func ProvideListConfig(ctx context.Context, filter []string, sort []string, outputDir string, outputCSV bool, includeDeletedFiles bool, fileName string) (*ListConfig, error) {
	log, err := util.InitLog()
	if err != nil {
		return nil, err
	}

	conn, err := util.InitClient(ctx)
	if err != nil {
		return nil, err
	}

	client := fileretriever.NewFileRetrieverClient(conn)

	if err != nil {
		return nil, err
	}

	return &ListConfig{
		Filter:              filter,
		Sort:                sort,
		OutputDir:           outputDir,
		OutputCSV:           outputCSV,
		IncludeDeletedFiles: includeDeletedFiles,
		FileName:            fileName,
		log:                 log,
		client:              client,
		conn:                conn,
	}, nil

}

// closeConnection closes the grpc client connection
func (lc *ListConfig) closeConnection() {
	if lc != nil && lc.conn != nil {
		lc.conn.Close()
	}
}

type nopCloser struct{}

func (n nopCloser) Close() error { return nil }

func (lc *ListConfig) getWriter() (*tablewriter.Table, io.Closer, error) {
	if lc.OutputCSV {
		fp := filepath.Join(lc.OutputDir, lc.FileName)
		file, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return nil, nil, err
		}

		writer, err := tablewriter.NewCSV(file, fp, true)

		if err != nil {
			return nil, nil, err
		}

		return writer, file, nil
	}

	return tablewriter.NewWriter(os.Stdout), nopCloser{}, nil
}

// listFileMetadata fetch list of files and its metadata from the grpc server to a specified directory
func (lc *ListConfig) listFileMetadata(ctx context.Context) error {
	var err error
	var filterList []*fileretriever.ListFileMetadataRequest_ListFileFilter
	var sortList []*fileretriever.ListFileMetadataRequest_ListFileSort
	var noOfRow int

	filterList, err = parseFilter(lc.Filter)
	if err != nil {
		return err
	}
	sortList, err = parseSort(lc.Sort)
	if err != nil {
		return err
	}

	req := &fileretriever.ListFileMetadataRequest{
		FilterBy:            filterList,
		SortBy:              sortList,
		IncludeDeletedFiles: lc.IncludeDeletedFiles,
	}

	//response, err := client.ListFileMetadata(ctx, req)
	result, err := lc.client.ListFileMetadata(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to retrieve list due to: %v", err)
	}

	var table *tablewriter.Table

	table, close, err := lc.getWriter()
	if err != nil {
		return err
	}
	defer close.Close()

	table.SetHeader(getHeaders())
	table.SetRowLine(true)

	for {
		resp, err := result.Recv()

		if err == io.EOF {
			break
		}

		lc.log.Info("Received response:", "info", resp)

		if resp.Results != nil {
			row, parseErr := parseFileInfo(resp.Results)

			if parseErr != nil {
				return parseErr
			}
			table.Append(row)
			noOfRow = noOfRow + 1
		}
	}

	table.SetCaption(true, "Files found: "+strconv.FormatInt(int64(noOfRow), 10))
	table.Render()
	return nil
}

// parseFilter parses arguments for filter operation
// It returns list of struct ListFileMetadataRequest_ListFileFilter and error if occured
func parseFilter(filter []string) ([]*fileretriever.ListFileMetadataRequest_ListFileFilter, error) {
	var listFilter []*fileretriever.ListFileMetadataRequest_ListFileFilter

	modelColumnSet := map[string]bool{
		"provided_name": true,
		"provided_id":   true,
		"size":          true,
		"created_at":    true,
		"deleted_at":    true,
	}

	for _, filterString := range filter {

		filterArgs := parseArgs(filterString)

		if len(filterArgs) != 3 {
			return nil,
				fmt.Errorf("'%v' : invalid number of arguments provided for filter operation, Required 3 | Provided %v ",
					filterString, len(filterArgs))
		}

		operator, err := parseFilterOperator(filterArgs[1], modelColumnSet[filterArgs[0]])
		if err != nil {
			return nil, err
		}

		var value string
		if filterArgs[0] == "created_at" || filterArgs[0] == "deleted_at" {
			value, err = parseDateToEpoch(filterArgs[2])
			if err != nil {
				return nil, err
			}
		} else {
			value = filterArgs[2]
		}

		req := &fileretriever.ListFileMetadataRequest_ListFileFilter{
			Key:      filterArgs[0],
			Operator: *operator,
			Value:    value,
		}
		listFilter = append(listFilter, req)
	}
	return listFilter, nil
}

// parseFilterOperator parses filter operator and returns respective operator defined by fileretriever.proto
func parseFilterOperator(op string, isColumn bool) (*fileretriever.ListFileMetadataRequest_ListFileFilter_Comparison, error) {
	var operator fileretriever.ListFileMetadataRequest_ListFileFilter_Comparison
	if isColumn {
		switch op {
		case "EQUAL":
			operator = fileretriever.ListFileMetadataRequest_ListFileFilter_EQUAL
		case "LESS_THAN":
			operator = fileretriever.ListFileMetadataRequest_ListFileFilter_LESS_THAN
		case "GREATER_THAN":
			operator = fileretriever.ListFileMetadataRequest_ListFileFilter_GREATER_THAN
		case "CONTAINS":
			operator = fileretriever.ListFileMetadataRequest_ListFileFilter_CONTAINS
		default:
			return nil, fmt.Errorf("invalid filter operation used: %v ", op)
		}
	} else {
		switch op {
		case "EQUAL":
			operator = fileretriever.ListFileMetadataRequest_ListFileFilter_EQUAL
		case "CONTAINS":
			operator = fileretriever.ListFileMetadataRequest_ListFileFilter_CONTAINS
		default:
			return nil, fmt.Errorf("invalid filter operation used: %v ", op)
		}
	}

	return &operator, nil
}

// parseSort parses arguments for sort operation
// It returns list of struct ListFileMetadataRequest_ListFileSort and error id occured
func parseSort(sortList []string) ([]*fileretriever.ListFileMetadataRequest_ListFileSort, error) {
	var listSort []*fileretriever.ListFileMetadataRequest_ListFileSort

	modelColumnSet := map[string]bool{
		"provided_name": true,
		"provided_id":   true,
		"size":          true,
		"created_at":    true,
		"deleted_at":    true,
	}

	for _, sortString := range sortList {

		sortArgs := parseArgs(sortString)

		if len(sortArgs) != 2 {
			return nil, fmt.Errorf("'%v' : invalid number of arguments provided for sort operation, Required 2 | Provided %v ", sortString, len(sortArgs))
		}
		if !modelColumnSet[sortArgs[0]] {
			return nil, fmt.Errorf("invalid operand passed for sort operation: %v ", sortArgs[0])
		}
		operator, err := parseSortOperator(sortArgs[1])
		if err != nil {
			return nil, err
		}
		req := &fileretriever.ListFileMetadataRequest_ListFileSort{
			Key:       sortArgs[0],
			SortOrder: *operator,
		}
		listSort = append(listSort, req)
	}
	return listSort, nil
}

// parseSortOperator parses sort operator and returns respective operator defined by fileretriever.proto
func parseSortOperator(op string) (*fileretriever.ListFileMetadataRequest_ListFileSort_SortOrder, error) {
	var operator fileretriever.ListFileMetadataRequest_ListFileSort_SortOrder

	switch op {
	case "ASC":
		operator = fileretriever.ListFileMetadataRequest_ListFileSort_ASC
	case "DESC":
		operator = fileretriever.ListFileMetadataRequest_ListFileSort_DESC
	default:
		return nil, fmt.Errorf("invalid sort operation used: %v", op)
	}

	return &operator, nil
}

// parseDateToEpoch parses date argument
// It returns unix epoch of date entered and error if occured
// Date can be in RFC3339 format or yyyy-mm-dd format
// for any valid date in yyyy-mm-dd format time selected will be midnight in UTC
// Eg: Input date: 2021-04-13
//     Complete date-time in RFC3339 :  2021-04-13 00:00:00 +0000 UTC
//     Unix Epoch: 1618272000
func parseDateToEpoch(date string) (string, error) {

	const layout = "2006-01-02"
	dt, er1 := time.Parse(time.RFC3339, date)
	if er1 != nil {
		dt1, er2 := time.Parse(layout, date)
		if er2 != nil {
			return "", fmt.Errorf(er1.Error() + er2.Error())
		}
		dt = dt1
	}

	time_ := time.Date(dt.Year(),
		time.Month(dt.Month()),
		dt.Day(),
		dt.Hour(),
		dt.Minute(),
		dt.Second(),
		dt.Nanosecond(),
		time.UTC)
	return strconv.FormatInt(time_.Unix(), 10), nil
}

// parseArgs takes double quoted string as input and return slice of string
// group of characters between two consecutive spaces will be considered as string
// any characters between single quotation marks will be grouped as string
func parseArgs(argString string) []string {
	var args []string
	var bstr []byte
	isQuoted := false
	for i, c := range argString {
		if string(c) == "'" {
			isQuoted = !isQuoted
			st := strings.TrimSpace(string(bstr))
			if len(st) != 0 {
				args = append(args, st)
			}
			bstr = []byte{}
			continue
		}
		if isQuoted {
			bstr = append(bstr, byte(c))
			if i == (len(argString) - 1) {
				st := strings.TrimSpace(string(bstr))
				if len(st) != 0 {
					args = append(args, st)
				}
			}
		} else {
			if string(c) == " " {
				st := strings.TrimSpace(string(bstr))
				if len(st) != 0 {
					args = append(args, st)
				}
				bstr = []byte{}
				continue
			}
			bstr = append(bstr, byte(c))
			if i == (len(argString) - 1) {
				st := strings.TrimSpace(string(bstr))
				if len(st) != 0 {
					args = append(args, st)
				}
			}
		}
	}
	return args
}

//parseFileInfo parses v1.Finfo and return slice of string
func parseFileInfo(finfo *v1.FileInfo) ([]string, error) {
	mdata, err := json.Marshal(finfo.Metadata)
	if err != nil {
		fmt.Println("foo")
		return nil, err
	}
	metadata := string(mdata)

	var deletedAt string
	if finfo.DeletedTombstone != nil {
		deletedAt = time.Unix(finfo.DeletedTombstone.Seconds, 0).Format(time.RFC3339)
	}

	return []string{
		finfo.FileId.GetId(),
		finfo.FileId.GetName(),
		strconv.FormatUint(uint64(finfo.GetSize()), 10),
		time.Unix(finfo.CreatedAt.Seconds, 0).Format(time.RFC3339),
		deletedAt,
		strconv.FormatBool(finfo.Compression),
		finfo.CompressionType,
		metadata,
		finfo.Checksum,
	}, nil
}

//writeToCSV writes stream output to csv
func writeToCSV(row []string, writer *csv.Writer) error {
	err := writer.Write(row)
	if err != nil {
		return err
	}
	return nil
}

//getHeaders returns headers for csv/table
func getHeaders() []string {
	return []string{
		"File ID",
		"File Name",
		"Size",
		"Created At",
		"Deleted At",
		"Compression",
		"Compression Type",
		"Metadata",
		"Checksum",
	}
}
