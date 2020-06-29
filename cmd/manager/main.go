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
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

var MarketplaceControllerSet = wire.NewSet(
	controller.ControllerSet,
	controller.ProvideControllerFlagSet,
	controller.SchemeDefinitions,
	makeMarketplaceController,
	reconcileutils.ProvideDefaultCommandRunnerProvider,
	wire.Bind(new(reconcileutils.ClientCommandRunnerProvider), new(*reconcileutils.DefaultCommandRunnerProvider)),
)

func makeMarketplaceController(
	controllerFlags *controller.ControllerFlagSet,
	controllerList controller.ControllerList,
	localSchemes controller.LocalSchemes,
) *managers.ControllerMain {
	return &managers.ControllerMain{
		Name: "redhat-marketplace-operator",
		FlagSets: []*pflag.FlagSet{
			(*pflag.FlagSet)(controllerFlags),
		},
		Controllers: controllerList,
		Schemes: localSchemes,
	}
}

func main() {
	marketplaceController := InitializeMarketplaceController()
	marketplaceController.Run()
}
