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

package manifests

import (
	"fmt"
	"io"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	v1 "k8s.io/api/core/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

type Config struct {
	RelatedImages            config.RelatedImages      `json:"relatedImages"`
	PrometheusOperatorConfig *PrometheusOperatorConfig `json:"prometheusOperator"`
	PrometheusConfig         *PrometheusConfig         `json:"prometheusConfig"`
	Platform                 configv1.PlatformType     `json:"-"`
}

type PrometheusOperatorConfig struct {
	ServiceAccountName string            `json:"serviceAccountName"`
	LogLevel           string            `json:"logLevel"`
	NodeSelector       map[string]string `json:"nodeSelector"`
	Tolerations        []v1.Toleration   `json:"tolerations"`
}

type PrometheusConfig struct {
	Retention string `json:"retention"`
}

type RelatedImages struct {
	Reporter      string
	MetricState   string
	KubeRbacProxy string
}

func (c *Config) LoadPlatform(load func() (*configv1.Infrastructure, error)) error {
	i, err := load()
	if err != nil {
		return fmt.Errorf("error loading platform: %v", err)
	}
	c.Platform = i.Status.Platform
	return nil
}

func NewConfig(content io.Reader) (*Config, error) {
	c := Config{}
	err := k8syaml.NewYAMLOrJSONDecoder(content, 4096).Decode(&c)
	if err != nil {
		return nil, err
	}
	res := &c
	res.applyDefaults()
	return res, nil
}

func NewOperatorConfig(cfg config.OperatorConfig) *Config {
	c := &Config{}
	c.RelatedImages = cfg.RelatedImages
	c.applyDefaults()
	return c
}

func (c *Config) applyDefaults() {
	if c.PrometheusOperatorConfig == nil {
		c.PrometheusOperatorConfig = &PrometheusOperatorConfig{}
	}

	if c.PrometheusConfig == nil {
		c.PrometheusConfig = &PrometheusConfig{}
		c.PrometheusConfig.Retention = "30d"
	}
}
