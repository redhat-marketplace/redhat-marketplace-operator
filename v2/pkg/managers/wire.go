// The build tag makes sure the stub is not built in the final build.

// +build wireinject

package managers

import (
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/runnables"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
)

func initializeRunnables(
	fields *ControllerFields,
	namespace DeployedNamespace,
) (runnables.Runnables, error) {
	panic(wire.Build(
		ProvideManagerSet,
		runnables.RunnableSet,
		reconcileutils.NewClientCommand,
		ProvidePodMonitorConfig,
	))
}

func initializeCommandRunner(fields *ControllerFields) (reconcileutils.ClientCommandRunner, error) {
	panic(wire.Build(
		ProvideManagerSet,
		reconcileutils.NewClientCommand,
	))
}
