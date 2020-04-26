package controller

import (
	"github.com/google/wire"
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type ControllerDefinition struct {
	Add     func(mgr manager.Manager) error
	FlagSet func() *pflag.FlagSet
}

var ControllerSet = wire.NewSet(
	ProvideMarketplaceController,
	ProvideMeterbaseController,
	ProvideRazeeDeployController,
	ProvideMeterDefinitionController,
	ProvideOlmSubscriptionController,
)
