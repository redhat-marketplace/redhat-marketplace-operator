package controller

import (
	"github.com/spf13/pflag"
)

var (
	controllerFlagSet *pflag.FlagSet

	assetsDirectory string
)

func FlagSet() *pflag.FlagSet {
	return controllerFlagSet
}

func AddFlag(flag *pflag.Flag) {
	controllerFlagSet.AddFlag(flag)
}

func AddFlagSet(flags *pflag.FlagSet) {
	controllerFlagSet.AddFlagSet(flags)
}

func init() {
	controllerFlagSet = pflag.NewFlagSet("controller", pflag.ExitOnError)
	controllerFlagSet.StringVar(&assetsDirectory, "asset-path", "./assets",
		"Set the assets directory that contains files for deployment")
}
