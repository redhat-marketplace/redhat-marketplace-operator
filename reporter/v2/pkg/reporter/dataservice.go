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

package reporter

import (
	"fmt"
	"io/ioutil"

	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
)

func provideDataServiceConfig(
	reporterConfig *Config,
) (*dataservice.DataServiceConfig, error) {
	deployedNamespace := reporterConfig.DeployedNamespace
	dataServiceTokenFile := reporterConfig.DataServiceTokenFile
	dataServiceCertFile := reporterConfig.DataServiceCertFile

	cert, err := ioutil.ReadFile(dataServiceCertFile)
	if err != nil {
		return nil, err
	}

	var serviceAccountToken = ""
	if dataServiceTokenFile != "" {
		content, err := ioutil.ReadFile(dataServiceTokenFile)
		if err != nil {
			return nil, err
		}
		serviceAccountToken = string(content)
	}

	var dataServiceDNS = fmt.Sprintf("%s.%s.svc:8004", utils.DATA_SERVICE_NAME, deployedNamespace)

	return &dataservice.DataServiceConfig{
		Address:          dataServiceDNS,
		DataServiceToken: serviceAccountToken,
		DataServiceCert:  cert,
		OutputPath:       reporterConfig.OutputDirectory,
	}, nil
}

// Per Dial token vs per Call
func provideGRPCDialOptions(
	dataServiceConfig *dataservice.DataServiceConfig,
) []grpc.DialOption {

	options := []grpc.DialOption{}

	if dataServiceConfig.DataServiceToken != "" {
		/* create oauth2 token  */
		oauth2Token := &oauth2.Token{
			AccessToken: dataServiceConfig.DataServiceToken,
		}

		perRPC := oauth.NewOauthAccess(oauth2Token)

		options = append(options, grpc.WithPerRPCCredentials(perRPC))
	}
	return options
}
