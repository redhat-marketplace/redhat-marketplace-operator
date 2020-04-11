package main

import (
	"github.com/google/wire"
	"github.com/spf13/pflag"

	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/managers"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller"
)

var MarketplaceControllerSet = wire.NewSet(
	controller.ControllerSet,
	controller.ProvideControllerFlagSet,
	managers.SchemeDefinitions,
	makeMarketplaceController,
)

func makeMarketplaceController(
	myController *controller.MarketplaceController,
	meterbaseC *controller.MeterbaseController,
	meterDefinitionC *controller.MeterDefinitionController,
	razeeC *controller.RazeeDeployController,
	controllerFlags *controller.ControllerFlagSet,
	opsSrcScheme *managers.OpsSrcSchemeDefinition,
	monitoringScheme *managers.MonitoringSchemeDefinition,
) *managers.ControllerMain {
	return &managers.ControllerMain{
		Name: "redhat-marketplace-operator",
		FlagSets: []*pflag.FlagSet{
			(*pflag.FlagSet)(controllerFlags),
		},
		Controllers: []*controller.ControllerDefinition{
			(*controller.ControllerDefinition)(myController),
			(*controller.ControllerDefinition)(meterbaseC),
			(*controller.ControllerDefinition)(meterDefinitionC),
			(*controller.ControllerDefinition)(razeeC),
		},
		Schemes: []*managers.SchemeDefinition{
			(*managers.SchemeDefinition)(monitoringScheme),
			(*managers.SchemeDefinition)(opsSrcScheme),
		},
	}
}

func main() {
	marketplaceController := initializeMarketplaceController()
	marketplaceController.Run()
}
