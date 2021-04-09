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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/runnables"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/certificates"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func initializeInjectDependencies(
	cache cache.Cache,
	fields *managers.ControllerFields,
) (injectorDependencies, error) {
	panic(wire.Build(
		managers.ProvideManagerSet,
		runnables.RunnableSet,
		reconcileutils.NewClientCommand,
		config.ProvideInfrastructureAwareConfig,
		ProvideInjectables,
		wire.Struct(new(ClientCommandInjector), "*"),
		wire.Struct(new(OperatorConfigInjector), "*"),
		wire.Struct(new(PatchInjector), "*"),
		wire.Struct(new(FactoryInjector), "*"),
		wire.Struct(new(KubeInterfaceInjector), "*"),
		wire.Struct(new(CertIssuerInjector), "*"),
		wire.Struct(new(injectorDependencies), "*"),
		ProvideNamespace,
		manifests.NewFactory,
		utils.NewCertIssuer,
	))
}

func initializeCommandRunner(fields *managers.ControllerFields) (reconcileutils.ClientCommandRunner, error) {
	panic(wire.Build(
		managers.ProvideManagerSet,
		reconcileutils.NewClientCommand,
	))
}
