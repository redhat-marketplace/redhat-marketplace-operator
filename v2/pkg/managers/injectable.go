package managers

import (
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrl "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Injectable interface {
	SetCustomFields(i interface{}) error
}

type Injectables []Injectable

func ProvideInjectables() []Injectable {
	return []Injectable{
		&InjectClientCommand{},
	}
}

type InjectableManager struct {
	ctrl.Manager
	*builder.Builder
	*builder.WebhookBuilder

	injectables Injectables
	fields      *ControllerFields
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
	deployed DeployedNamespace,
) (*InjectableManager, error) {
	fields := &ControllerFields{}
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

	return &InjectableManager{
		Manager:        mgr,
		Builder:        builder.ControllerManagedBy(mgr),
		WebhookBuilder: builder.WebhookManagedBy(mgr),
		fields:         fields,
		injectables:    ProvideInjectables(),
	}, nil
}

type InjectClientCommand interface {
	InjectCommandRunner(ccp ClientCommandRunner)
}

type InjectOperatorConfig interface {
}

type InjectPatcher interface {
}

type ClientCommandInjector struct{}

func (a *ClientCommandInjector) SetCustomFields(i interface{}) error {
	if ii, ok := i.(InjectClientCommandRunner); ok {
		runner, err := initializeCommandRunner(a.fields)

		if err != nil {
			return err
		}

		ii.InjectReconcileCommandRunner(runner)
	}
	return nil
}
