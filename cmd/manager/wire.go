// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
)

func initializeMarketplaceController() *managers.ControllerMain {
	panic(wire.Build(MarketplaceControllerSet))
}
