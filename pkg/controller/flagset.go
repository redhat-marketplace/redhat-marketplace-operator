package controller

import (
	"github.com/spf13/pflag"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/utils"
)

var (
	controllerFlagSet *pflag.FlagSet
	flagSets          []*pflag.FlagSet
)

func init() {
	controllerFlagSet = pflag.NewFlagSet("controller", pflag.ExitOnError)
	controllerFlagSet.String("assets", utils.Getenv("ASSETS", "./assets"),
		"Set the assets directory that contains files for deployment")
	flagSets = append(flagSets, controllerFlagSet)
}

func FlagSets() []*pflag.FlagSet {
	return flagSets
}
