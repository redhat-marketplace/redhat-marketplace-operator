package main

import (
	"github.com/google/wire"
	"github.com/spf13/pflag"

	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/managers"
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
	olmSubscriptionC *controller.OlmSubscriptionController,
	controllerFlags *controller.ControllerFlagSet,
	opsSrcScheme *managers.OpsSrcSchemeDefinition,
	monitoringScheme *managers.MonitoringSchemeDefinition,
	olmv1Scheme *managers.OlmV1SchemeDefinition,
	olmv1alphaScheme *managers.OlmV1Alpha1SchemeDefinition,
) *managers.ControllerMain {
	return &managers.ControllerMain{
		Name: "redhat-marketplace-operator",
		FlagSets: []*pflag.FlagSet{
			(*pflag.FlagSet)(controllerFlags),
		},
		Controllers: []*controller.ControllerDefinition{
			(*controller.ControllerDefinition)(myController),
			//(*controller.ControllerDefinition)(meterbaseC),
			//(*controller.ControllerDefinition)(meterDefinitionC),
			(*controller.ControllerDefinition)(razeeC),
			(*controller.ControllerDefinition)(olmSubscriptionC),
		},
		Schemes: []*managers.SchemeDefinition{
			(*managers.SchemeDefinition)(monitoringScheme),
			(*managers.SchemeDefinition)(opsSrcScheme),
			(*managers.SchemeDefinition)(olmv1Scheme),
			(*managers.SchemeDefinition)(olmv1alphaScheme),
		},
	}
}

func main() {
	marketplaceController := initializeMarketplaceController()
	marketplaceController.Run()
}
