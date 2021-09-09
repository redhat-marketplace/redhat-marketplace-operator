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

package server

import (
	"context"
	"fmt"

	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/adminserver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AdminServerServer struct {
	adminserver.UnimplementedAdminServerServer
	B BaseServer
}

// CleanTombstones allows us to clear file contents/ delete file records
func (frs *AdminServerServer) CleanTombstones(ctx context.Context, in *adminserver.CleanTombstonesRequest) (*adminserver.CleanTombstonesResponse, error) {

	fileList, err := frs.B.FileStore.CleanTombstones(in.GetBefore(), in.GetPurgeAll())
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Failed to clean tombstoned files from database due to: %v", err),
		)
	}

	res := &adminserver.CleanTombstonesResponse{
		TombstonesCleaned: int32(len(fileList)),
		Files:             fileList,
	}
	return res, nil
}

func (frs *AdminServerServer) DeleteFile(ctx context.Context, dfr *adminserver.DeleteFileRequest) (*adminserver.DeleteFileResponse, error) {

	err := frs.B.FileStore.TombstoneFile(dfr.GetFileId())
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Failed to mark file for deletion due to: %v", err),
		)
	}

	res := &adminserver.DeleteFileResponse{
		FileId: dfr.GetFileId(),
	}
	return res, nil
}
