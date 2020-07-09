package manifests

import (
	"bytes"
	"fmt"
	"io"

	configv1 "github.com/openshift/api/config/v1"
	v1 "k8s.io/api/core/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

type Config struct {
	MarketplaceOperatorConfig *MarketplaceOperatorConfig `json:"-"`
	Platform                  configv1.PlatformType      `json:"-"`
}

type MarketplaceOperatorConfig struct {
	PrometheusOperatorConfig *PrometheusOperatorConfig `json:"prometheusOperator"`
}

type PrometheusOperatorConfig struct {
	ServiceAccountName string            `json:"serviceAccountName"`
	LogLevel           string            `json:"logLevel"`
	NodeSelector       map[string]string `json:"nodeSelector"`
	Tolerations        []v1.Toleration   `json:"tolerations"`
}

func (c *Config) LoadPlatform(load func() (*configv1.Infrastructure, error)) error {
	i, err := load()
	if err != nil {
		return fmt.Errorf("error loading platform: %v", err)
	}
	c.Platform = i.Status.Platform
	return nil
}

func NewConfigFromString(content string) (*Config, error) {
	if content == "" {
		return NewDefaultConfig(), nil
	}

	return NewConfig(bytes.NewBuffer([]byte(content)))
}

func NewConfig(content io.Reader) (*Config, error) {
	c := Config{}
	moc := MarketplaceOperatorConfig{}
	err := k8syaml.NewYAMLOrJSONDecoder(content, 4096).Decode(&moc)
	if err != nil {
		return nil, err
	}
	c.MarketplaceOperatorConfig = &moc
	res := &c
	res.applyDefaults()

	return res, nil
}

func NewDefaultConfig() *Config {
	c := &Config{}
	moc := MarketplaceOperatorConfig{}
	c.MarketplaceOperatorConfig = &moc
	c.applyDefaults()
	return c
}

func (c *Config) applyDefaults() {
	if c.MarketplaceOperatorConfig.PrometheusOperatorConfig == nil {
		c.MarketplaceOperatorConfig.PrometheusOperatorConfig = &PrometheusOperatorConfig{}
	}
}
