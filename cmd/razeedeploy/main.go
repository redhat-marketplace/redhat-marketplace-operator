package main

import (

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.ibm.com/symposium/redhat-marketplace-operator/cmd/managers"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller"

	"github.com/google/wire"
	"github.com/spf13/pflag"
)

var RazeeControllerSet = wire.NewSet(
	controller.ControllerSet,
	controller.ProvideControllerFlagSet,
	managers.SchemeDefinitions,
	makeController,
)

func makeController(
	razeeC *controller.RazeeDeployController,
	controllerFlags *controller.ControllerFlagSet,
) *managers.ControllerMain {
	return &managers.ControllerMain{
		Name: "redhat-marketplace-razeedeploy-operator",
		FlagSets: []*pflag.FlagSet{
			(*pflag.FlagSet)(controllerFlags),
		},
		Controllers: []*controller.ControllerDefinition{
			(*controller.ControllerDefinition)(razeeC),
		},
	}
}

func main() {
	razeeController := initializeRazeeController()
	razeeController.Run()
}
