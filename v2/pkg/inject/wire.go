// Copyright 2021 IBM Corp.
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

// The build tag makes sure the stub is not built in the final build.

// +build wireinject

package inject

import (
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/runnables"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
)

func initializeRunnables(
	fields *managers.ControllerFields,
	namespace managers.DeployedNamespace,
) (runnables.Runnables, error) {
	panic(wire.Build(
		managers.ProvideManagerSet,
		runnables.RunnableSet,
		reconcileutils.NewClientCommand,
		managers.ProvidePodMonitorConfig,
		config.ProvideInfrastructureAwareConfig,
	))
}

func initializeInjectables(
	fields *managers.ControllerFields,
	namespace managers.DeployedNamespace,
) (Injectables, error) {
	panic(wire.Build(
		ProvideInjectables,
		managers.ProvideManagerSet,
		reconcileutils.NewClientCommand,
		config.ProvideInfrastructureAwareConfig,
		wire.Struct(new(ClientCommandInjector), "*"),
		wire.Struct(new(OperatorConfigInjector), "*"),
		wire.Struct(new(PatchInjector), "*"),
		wire.Struct(new(FactoryInjector), "*"),
	))
}

func initializeCommandRunner(fields *managers.ControllerFields) (reconcileutils.ClientCommandRunner, error) {
	panic(wire.Build(
		managers.ProvideManagerSet,
		reconcileutils.NewClientCommand,
	))
}
