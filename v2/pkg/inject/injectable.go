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

package inject

import (
	"github.com/pkg/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/marketplace"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/runnables"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

var injectLog = ctrl.Log.WithName("injector")

type Injectables []types.Injectable

func ProvideInjectables(
	i1 *ClientCommandInjector,
	i2 *OperatorConfigInjector,
	i3 *PatchInjector,
	i4 *FactoryInjector,
	i5 *KubeInterfaceInjector,
) Injectables {
	return []types.Injectable{i1, i2, i3, i4, i5}
}

type Injector struct {
	injectables Injectables
	fields      *managers.ControllerFields
}

func (a *Injector) SetCustomFields(i interface{}) error {
	injectLog.Info("setting custom field")
	for _, inj := range a.injectables {
		if err := inj.SetCustomFields(i); err != nil {
			return err
		}
	}
	return nil
}

type injectorDependencies struct {
	runnables.Runnables
	Injectables
}

func ProvideNamespace(cfg *config.OperatorConfig) managers.DeployedNamespace {
	return managers.DeployedNamespace(cfg.DeployedNamespace)
}

func ProvideInjector(
	mgr ctrl.Manager,
) (*Injector, error) {
	fields := &managers.ControllerFields{}
	if err := mgr.SetFields(fields); err != nil {
		return nil, errors.Wrap(err, "failed set fields")
	}

	dependencies, err := initializeInjectDependencies(mgr.GetCache(), fields)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init dependencies")
	}

	for _, runnable := range dependencies.Runnables {
		err := mgr.Add(runnable)

		if err != nil {
			return nil, errors.Wrap(err, "failed add runnables")
		}
	}

	return &Injector{
		fields:      fields,
		injectables: dependencies.Injectables,
	}, nil
}

type CommandRunner interface {
	InjectCommandRunner(reconcileutils.ClientCommandRunner) error
}

type OperatorConfig interface {
	InjectOperatorConfig(*config.OperatorConfig) error
}

type Patch interface {
	InjectPatch(patch.Patcher) error
}

type Factory interface {
	InjectFactory(*manifests.Factory) error
}

type KubeInterface interface {
	InjectKubeInterface(kubernetes.Interface) error
}

type MarketplaceClientBuilder interface {
	InjectMarketplaceClientBuilder(marketplace.MarketplaceClientBuilder) error
}

type ClientCommandInjector struct {
	Fields        *managers.ControllerFields
	CommandRunner reconcileutils.ClientCommandRunner
}

func (a *ClientCommandInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(CommandRunner); ok {
		return ii.InjectCommandRunner(a.CommandRunner)
	}
	return nil
}

type PatchInjector struct{}

func (a *PatchInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(Patch); ok {
		return ii.InjectPatch(patch.RHMDefaultPatcher)
	}
	return nil
}

type OperatorConfigInjector struct {
	Config *config.OperatorConfig
}

func (a *OperatorConfigInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(OperatorConfig); ok {
		return ii.InjectOperatorConfig(a.Config)
	}
	return nil
}

type FactoryInjector struct {
	Fields    *managers.ControllerFields
	Config    *config.OperatorConfig
	Namespace managers.DeployedNamespace
	Scheme    *runtime.Scheme
	*manifests.Factory
}

func (a *FactoryInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(Factory); ok {
		return ii.InjectFactory(a.Factory)
	}
	return nil
}

type KubeInterfaceInjector struct {
	KubeInterface kubernetes.Interface
}

func (a *KubeInterfaceInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(KubeInterface); ok {
		return ii.InjectKubeInterface(a.KubeInterface)
	}
	return nil
}

type MarketplaceClientBuilderInjector struct {
	MarketplaceClientBuilder marketplace.MarketplaceClientBuilder
}

func (a *MarketplaceClientBuilderInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(MarketplaceClientBuilder); ok {
		return ii.InjectMarketplaceClientBuilder(a.MarketplaceClientBuilder)
	}
	return nil
}
