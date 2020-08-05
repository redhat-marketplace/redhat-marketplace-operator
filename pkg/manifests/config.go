package manifests

import (
	"bytes"
	"fmt"
	"io"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/config"
	v1 "k8s.io/api/core/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

type Config struct {
	RelatedImages            *config.RelatedImages     `json:"relatedImages"`
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

func NewConfigFromString(content string) (*Config, error) {
	if content == "" {
		return NewDefaultConfig(), nil
	}

	return NewConfig(bytes.NewBuffer([]byte(content)))
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

func NewDefaultConfig() *Config {
	cfg, _ := config.ProvideConfig()
	c := &Config{}
	c.RelatedImages = &cfg.RelatedImages
	c.applyDefaults()
	return c
}

func NewOperatorConfig(cfg *config.OperatorConfig) *Config {
	c := &Config{}
	c.RelatedImages = &cfg.RelatedImages
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
