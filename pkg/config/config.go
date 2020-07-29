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
