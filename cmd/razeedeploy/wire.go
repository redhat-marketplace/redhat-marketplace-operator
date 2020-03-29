// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/google/wire"
	"github.ibm.com/symposium/redhat-marketplace-operator/cmd/managers"
)

func initializeRazeeController() *managers.ControllerMain {
	panic(wire.Build(RazeeControllerSet))
}
