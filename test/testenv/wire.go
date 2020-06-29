// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package testenv

import (
	"github.com/google/wire"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

var TestControllerSet = wire.NewSet(
	ControllerSet,
	ProvideControllerFlagSet,
	SchemeDefinitions,
	reconcileutils.ProvideDefaultCommandRunnerProvider,
	wire.Bind(new(reconcileutils.ClientCommandRunnerProvider), new(*reconcileutils.DefaultCommandRunnerProvider)),
)

func initializeLocalSchemes() LocalSchemes {
	panic(wire.Build(TestControllerSet))
}

func initializeControllers() ControllerList {
	panic(wire.Build(TestControllerSet))
}
