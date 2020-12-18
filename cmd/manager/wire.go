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

// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/go-logr/logr"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers/runnables"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

func InitializeMarketplaceController() (*managers.ControllerMain, error) {
	panic(wire.Build(
		config.ProvideInfrastructureAwareConfig,
		controller.ControllerSet,
		controller.ProvideControllerFlagSet,
		controller.SchemeDefinitions,
		reconcileutils.CommandRunnerProviderSet,
		managers.ProvideManagerSet,
		runnables.NewPodMonitor,
		providePodMonitorConfig,
		wire.InterfaceValue(new(logr.Logger), logger),
		makeMarketplaceController,
		provideOptions,
	))
}
