package main

import (
	"github.com/google/wire"
	"github.com/spf13/pflag"

	"github.ibm.com/symposium/redhat-marketplace-operator/cmd/managers"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller"
)

var MarketplaceControllerSet = wire.NewSet(
	controller.ProvideMarketplaceController,
	controller.ProvideControllerFlagSet,
	managers.SchemeDefinitions,
	makeMarketplaceController,
)

func makeMarketplaceController(
	myController *controller.MarketplaceController,
	controllerFlags *controller.ControllerFlagSet,
	opsSrcScheme *managers.OpsSrcSchemeDefinition,
) *managers.ControllerMain {
	return &managers.ControllerMain{
		Name: "redhat-marketplace-operator",
		FlagSets: []*pflag.FlagSet{
			(*pflag.FlagSet)(controllerFlags),
		},
		Controllers: []*controller.ControllerDefinition{
			(*controller.ControllerDefinition)(myController),
		},
		Schemes: []*managers.SchemeDefinition{
			(*managers.SchemeDefinition)(opsSrcScheme),
		},
	}
}

func main() {
	marketplaceController := initializeMarketplaceController()
	marketplaceController.Run()
}
