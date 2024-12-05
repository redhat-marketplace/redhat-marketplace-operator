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
	"context"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	u "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/marketplace"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ProvideUploader(us u.Uploaders) (u.Uploader, error) {
	if len(us) == 0 {
		return nil, errors.New("uploader not provided")
	}
	return us[0], nil
}

func ProvideUploaders(
	ctx context.Context,
	client client.Client,
	log logr.Logger,
	reporterConfig *Config,
) (u.Uploaders, error) {
	uploaders := u.Uploaders{}

	for _, target := range reporterConfig.UploaderTargets {
		switch target.(type) {
		case *u.RedHatInsightsUploader:
			uploader, err := u.ProvideRedHatInsightsUploader(ctx, client, log)
			if err != nil {
				return uploaders, err
			}
			uploaders = append(uploaders, uploader)
		case *u.NoOpUploader:
			uploaders = append(uploaders, target.(u.Uploader))
		case *u.LocalFilePathUploader:
			uploaders = append(uploaders, target.(u.Uploader))
		case *dataservice.DataService:
			dataServiceConfig, err := provideDataServiceConfig(reporterConfig)
			if err != nil {
				return nil, err
			}

			uploader, err := dataservice.NewDataService(dataServiceConfig)
			if err != nil {
				return uploaders, err
			}

			// Set Token
			opts, err := dataservice.ProvideGRPCCallOptions(reporterConfig.DataServiceTokenFile)
			if err != nil {
				return uploaders, err
			}
			uploader.SetCallOpts(opts...)

			uploaders = append(uploaders, uploader)
		case *u.COSS3Uploader:
			cosS3Config, err := provideCOSS3Config(ctx, client, reporterConfig.DeployedNamespace, log)
			if err != nil {
				return nil, err
			}

			uploader, err := u.NewCOSS3Uploader(cosS3Config)
			if err != nil {
				return uploaders, err
			}
			uploaders = append(uploaders, uploader)
		case *u.MarketplaceUploader:
			config, err := provideMarketplaceConfig(ctx,
				client,
				reporterConfig,
				log)
			// No secret is acceptable in disconnected environment
			if err == utils.NoSecretsFound && reporterConfig.IsDisconnected {
				log.Info("Disconnected mode, no redhat-marketplace-pull-secret or ibm-entitlement-key secret found, MarketplaceUploader will be unavailable")
			} else if err != nil {
				return nil, err
			} else {
				uploader, err := u.NewMarketplaceUploader(config)
				if err != nil {
					return nil, err
				}
				uploaders = append(uploaders, uploader)
			}
		default:
			return nil, errors.Errorf("uploader target not available %s", target.Name())
		}
	}

	return uploaders, nil
}

func provideCOSS3Config(
	ctx context.Context,
	client client.Client,
	deployedNamespace string,
	log logr.Logger,
) (*uploaders.COSS3UploaderConfig, error) {
	secret := &corev1.Secret{}

	if err := client.Get(ctx, types.NamespacedName{
		Name:      utils.RHM_COS_UPLOADER_SECRET,
		Namespace: deployedNamespace,
	}, secret); err != nil {
		return nil, err
	}

	configYamlBytes, ok := secret.Data["config.yaml"]
	if !ok {
		return nil, errors.New("rhm-cos-uploader-secret does not contain a config.yaml")
	}

	cosS3UploaderConfig := &uploaders.COSS3UploaderConfig{}

	err := yaml.Unmarshal(configYamlBytes, &cosS3UploaderConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal rhm-cos-uploader-secret config.yaml")
	}

	return cosS3UploaderConfig, nil
}

const (
	MktplProductionURL = "https://swc.saas.ibm.com"
	MktplStageURL      = "https://sandbox.swc.saas.ibm.com"
)

func provideMarketplaceConfig(
	ctx context.Context,
	client client.Client,
	reporterConfig *Config,
	log logr.Logger,
) (*uploaders.MarketplaceUploaderConfig, error) {
	log.Info("finding secret redhat-marketplace-pull-secret or ibm-entitlement-key")
	b := utils.ProvideSecretFetcherBuilder(client, ctx, reporterConfig.DeployedNamespace)
	si, err := b.ReturnSecret()
	if err != nil {
		return nil, err
	}

	log.Info("found secret", "secret name", si.Secret.GetName())

	marketplaceConfig, err := getMarketplaceConfig(ctx, client, reporterConfig)
	if err != nil {
		return nil, err
	}

	url := MktplProductionURL
	if si.Env == marketplace.EnvStage {
		url = MktplStageURL
	}

	return &uploaders.MarketplaceUploaderConfig{
		URL:                      url,
		AuthorizeAccountCreation: ptr.ToBool(marketplaceConfig.Spec.AuthorizeAccountCreation),
		Token:                    si.Token,
		CipherSuites:             reporterConfig.CipherSuites,
		MinVersion:               reporterConfig.MinVersion,
	}, nil
}
