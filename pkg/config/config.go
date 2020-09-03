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
	"strings"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/spf13/viper"
)

type OperatorConfig struct {
	RelatedImages `mapstructure:"related"`
}

type RelatedImages struct {
	Image struct {
		Reporter      string `mapstructure:"reporter"`
		KubeRbacProxy string `mapstructure:"kubeRbacProxy"`
		MetricState   string `mapstructure:"metricState"`
	}
}

func ProvideConfig() (*OperatorConfig, error) {
	cfg := &OperatorConfig{}

	replacer := strings.NewReplacer(
		".", "_",
	)
	viper.SetEnvPrefix("")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	err := viper.Unmarshal(cfg)
	if err != nil {
		return nil, err
	}

	cfg.RelatedImages.Image.Reporter = utils.Getenv("RELATED_IMAGE_REPORTER", "")
	cfg.RelatedImages.Image.KubeRbacProxy = utils.Getenv("RELATED_IMAGE_KUBE_RBAC_PROXY", "")
	cfg.RelatedImages.Image.MetricState = utils.Getenv("RELATED_IMAGE_METRIC_STATE", "")

	return cfg, nil
}
