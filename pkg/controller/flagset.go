package controller

import (
	"github.com/spf13/pflag"
)

var (
	controllerFlagSet *pflag.FlagSet
	flagSets []*pflag.FlagSet
)

func init() {
	controllerFlagSet = pflag.NewFlagSet("controller", pflag.ExitOnError)
	controllerFlagSet.String("asset-path", "./assets",
		"Set the assets directory that contains files for deployment")
	flagSets = append(flagSets, controllerFlagSet)
}

func FlagSets() []*pflag.FlagSet {
	return flagSets
}
