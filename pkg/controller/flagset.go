package controller

import (
	"github.com/spf13/pflag"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/utils"
)

type ControllerFlagSet pflag.FlagSet

func ProvideControllerFlagSet() *ControllerFlagSet {
	controllerFlagSet := &pflag.FlagSet{}
	controllerFlagSet = pflag.NewFlagSet("controller", pflag.ExitOnError)
	controllerFlagSet.String("assets", utils.Getenv("ASSETS", "./assets"),
		"Set the assets directory that contains files for deployment")
	controllerFlagSet.String("namespace", utils.Getenv("POD_NAMESPACE", ""),
		"Namespace for the controller")
	return (*ControllerFlagSet)(controllerFlagSet)
}
