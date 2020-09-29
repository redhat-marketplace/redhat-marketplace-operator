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

package config

import (
	"github.com/caarlos0/env/v6"
)

// OperatorConfig is the configuration for the operator
type OperatorConfig struct {
	RelatedImages
	Features
}

// RelatedImages stores relatedimages for the operator
type RelatedImages struct {
	Reporter      string `env:"RELATED_IMAGE_REPORTER" envDefault:"reporter:latest"`
	KubeRbacProxy string `env:"RELATED_IMAGE_KUBE_RBAC_PROXY" envDefault:"kube-proxy:latest"`
	MetricState   string `env:"RELATED_IMAGE_METRIC_STATE" envDefault:"metric-state:latest"`
	AuthChecker   string `env:"RELATED_IMAGE_AUTHCHECK" envDefault:"authcheck:latest"`
}

// Features store feature flags
type Features struct {
	IBMCatalog bool `env:"FEATURE_IBMCATALOG" envDefault:"true"`
}

// ProvideConfig gets the config from env vars
func ProvideConfig() (OperatorConfig, error) {
	cfg := OperatorConfig{}

	err := env.Parse(&cfg)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}
