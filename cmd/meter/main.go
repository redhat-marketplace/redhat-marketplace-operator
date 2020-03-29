package main

import (

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.ibm.com/symposium/redhat-marketplace-operator/cmd/managers"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller"

	"github.com/google/wire"
	"github.com/spf13/pflag"
)

var MeterControllerSet = wire.NewSet(
	controller.ControllerSet,
	controller.ProvideControllerFlagSet,
	managers.SchemeDefinitions,
	makeController,
)

func makeController(
	meterbaseC *controller.MeterbaseController,
	meterDefinitionC *controller.MeterDefinitionController,
	controllerFlags *controller.ControllerFlagSet,
	monitoringScheme *managers.MonitoringSchemeDefinition,
) *managers.ControllerMain {
	return &managers.ControllerMain{
		Name: "redhat-marketplace-meter-operator",
		FlagSets: []*pflag.FlagSet{
			(*pflag.FlagSet)(controllerFlags),
		},
		Controllers: []*controller.ControllerDefinition{
			(*controller.ControllerDefinition)(meterbaseC),
			(*controller.ControllerDefinition)(meterDefinitionC),
		},
		Schemes: []*managers.SchemeDefinition{
			(*managers.SchemeDefinition)(monitoringScheme),
		},
	}
}

func main() {
	meteringController := initializeMeterController()
	meteringController.Run()
}
