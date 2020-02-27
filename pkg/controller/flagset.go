package controller

import (
	"github.com/spf13/pflag"
	"github.ibm.com/symposium/marketplace-operator/pkg/utils"
)

var (
	controllerFlagSet *pflag.FlagSet
	flagSets          []*pflag.FlagSet
)

// init intializes the controllerFlagSet
// appends the controllerFlagSet to the array of flagSets
func init() {
	controllerFlagSet = pflag.NewFlagSet("controller", pflag.ExitOnError)
	controllerFlagSet.String("assets", utils.Getenv("ASSETS", "./assets"),
		"Set the assets directory that contains files for deployment")
	flagSets = append(flagSets, controllerFlagSet)
}

// FlagSets retunrs an array of type *FlagSet
func FlagSets() []*pflag.FlagSet {
	return flagSets
}
