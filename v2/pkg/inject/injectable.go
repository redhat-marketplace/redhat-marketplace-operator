package inject

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrl "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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

type InjectableManager struct {
	ctrl.Manager
	*builder.Builder
	*builder.WebhookBuilder

	injectables Injectables
	fields      *managers.ControllerFields
}

func (a *InjectableManager) SetCustomFields(i interface{}) error {
	for _, inj := range a.injectables {
		if err := inj.SetCustomFields(i); err != nil {
			return err
		}
	}
	return nil
}

func (a *InjectableManager) Complete(r reconcile.Reconciler) error {
	if err := a.SetCustomFields(a); err != nil {
		return err
	}
	return a.Builder.Complete(r)
}

func ProvideInjectableManager(
	mgr ctrl.Manager,
	deployed managers.DeployedNamespace,
) (*InjectableManager, error) {
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

	return &InjectableManager{
		Manager:        mgr,
		Builder:        builder.ControllerManagedBy(mgr),
		WebhookBuilder: builder.WebhookManagedBy(mgr),
		fields:         fields,
		injectables:    injs,
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

type OperatorConfigInjector struct{}

func (a *OperatorConfigInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(OperatorConfig); ok {
		cfg, err := config.GetConfig()

		if err != nil {
			return err
		}

		return ii.InjectOperatorConfig(cfg)
	}
	return nil
}

type FactoryInjector struct {
	Fields    *managers.ControllerFields
	Config    config.OperatorConfig
	Namespace managers.DeployedNamespace
}

func (a *FactoryInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(Factory); ok {
		f := manifests.NewFactory(string(a.Namespace), manifests.NewOperatorConfig(a.Config))

		return ii.InjectFactory(*f)
	}
	return nil
}
