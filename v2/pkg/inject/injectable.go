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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

var injectLog = ctrl.Log.WithName("injector")

type SetupWithManager interface {
	SetupWithManager(mgr ctrl.Manager) error
}

type Inject interface {
	Inject(injector *Injector) SetupWithManager
}

type Injectable interface {
	SetCustomFields(i interface{}) error
}

type Injectables []Injectable

func ProvideInjectables(
	i1 *ClientCommandInjector,
	i2 *OperatorConfigInjector,
	i3 *PatchInjector,
	i4 *FactoryInjector,
) Injectables {
	return []Injectable{i1, i2, i3, i4}
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

func ProvideInjector(
	mgr ctrl.Manager,
	deployed managers.DeployedNamespace,
) (*Injector, error) {
	fields := &managers.ControllerFields{}
	if err := mgr.SetFields(fields); err != nil {
		return nil, err
	}

	runnables, err := initializeRunnables(fields, deployed)
	if err != nil {
		return nil, err
	}

	for _, runnable := range runnables {
		err := mgr.Add(runnable)

		if err != nil {
			return nil, err
		}
	}

	injs, err := initializeInjectables(fields, deployed)

	if err != nil {
		return nil, err
	}

	return &Injector{
		fields:      fields,
		injectables: injs,
	}, nil
}

type CommandRunner interface {
	InjectCommandRunner(reconcileutils.ClientCommandRunner) error
}

type OperatorConfig interface {
	InjectOperatorConfig(config.OperatorConfig) error
}

type Patch interface {
	InjectPatch(patch.Patcher) error
}

type Factory interface {
	InjectFactory(manifests.Factory) error
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
	Config config.OperatorConfig
}

func (a *OperatorConfigInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(OperatorConfig); ok {
		return ii.InjectOperatorConfig(a.Config)
	}
	return nil
}

type FactoryInjector struct {
	Fields    *managers.ControllerFields
	Config    config.OperatorConfig
	Namespace managers.DeployedNamespace
	Scheme    *runtime.Scheme
}

func (a *FactoryInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(Factory); ok {
		f := manifests.NewFactory(string(a.Namespace), manifests.NewOperatorConfig(a.Config), &a.Config, a.Scheme)

		return ii.InjectFactory(*f)
	}
	return nil
}
