package config

import (
	"strings"

	"github.com/spf13/viper"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
)

type OperatorConfig struct {
	RelatedImages `mapstructure:"related"`
}

type RelatedImages struct {
	Image struct {
		Reporter string `mapstructure:"reporter"`
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

	return cfg, nil
}
