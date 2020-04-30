// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"github.com/google/wire"
	"github.com/spf13/pflag"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
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
			(*controller.ControllerDefinition)(meterbaseC),
			(*controller.ControllerDefinition)(meterDefinitionC),
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
