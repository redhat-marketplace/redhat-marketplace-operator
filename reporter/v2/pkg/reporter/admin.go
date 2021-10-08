// Copyright 2020 IBM Corp.
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

package reporter

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/adminserver"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
)

func (u *DataServiceAdmin) Name() string {
	return "data-service"
}

type Admin interface {
	DeleteFile(path string) error
}

type DataServiceAdmin struct {
	DataServiceConfig
	AdminServerClient adminserver.AdminServerClient
}

func NewDataServiceAdmin(ctx context.Context, dataServiceConfig *DataServiceConfig) (Admin, error) {
	adminServerClient, err := createDataServiceAdminClient(ctx, dataServiceConfig)
	if err != nil {
		return nil, err
	}

	return &DataServiceAdmin{
		AdminServerClient: adminServerClient,
		DataServiceConfig: *dataServiceConfig,
	}, nil
}

func createDataServiceAdminClient(ctx context.Context, dataServiceConfig *DataServiceConfig) (adminserver.AdminServerClient, error) {
	logger.Info("airgap url", "url", dataServiceConfig.Address)

	conn, err := newGRPCConn(ctx, dataServiceConfig.Address, dataServiceConfig.DataServiceCert, dataServiceConfig.DataServiceToken)

	if err != nil {
		logger.Error(err, "failed to establish connection")
		return nil, err
	}

	return adminserver.NewAdminServerClient(conn), nil
}

func (d *DataServiceAdmin) DeleteFile(path string) error {
	fn := strings.TrimSpace(path)
	var req *adminserver.DeleteFileRequest

	// Validate input and prepare request
	if len(fn) == 0 {
		return fmt.Errorf("file id/name is blank")
	} else {
		req = &adminserver.DeleteFileRequest{
			FileId: &v1.FileID{
				Data: &v1.FileID_Name{
					Name: fn},
			},
		}
	}

	_, err := d.AdminServerClient.DeleteFile(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to attempt delete due to: %v", err)
	}

	return nil
}

func ProvideAdmin(
	ctx context.Context,
	cc ClientCommandRunner,
	log logr.Logger,
	reporterConfig *Config,
) (Admin, error) {

	dataServiceConfig, err := provideDataServiceConfig(reporterConfig.DeployedNamespace, reporterConfig.DataServiceTokenFile, reporterConfig.DataServiceCertFile)
	if err != nil {
		return nil, err
	}

	return NewDataServiceAdmin(ctx, dataServiceConfig)
}
