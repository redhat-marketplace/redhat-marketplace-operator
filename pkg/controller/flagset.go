package controller

import (
	"github.com/spf13/pflag"
)

var (
	controllerFlagSet *pflag.FlagSet
	flagSets          []*pflag.FlagSet
)

// init intializes the controllerFlagSet
// appends the controllerFlagSet to the array of flagSets
func init() {
	controllerFlagSet = pflag.NewFlagSet("controller", pflag.ExitOnError)
	flagSets = append(flagSets, controllerFlagSet)
}

// FlagSets returns an array of type *FlagSet
func FlagSets() []*pflag.FlagSet {
	return flagSets
}
