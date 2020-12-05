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
		config.GetConfig,
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
