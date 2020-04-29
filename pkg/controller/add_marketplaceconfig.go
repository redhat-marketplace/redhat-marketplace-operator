package controller

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/marketplaceconfig"
)

type MarketplaceController ControllerDefinition

func ProvideMarketplaceController() *MarketplaceController {
	return &MarketplaceController{
		Add:     marketplaceconfig.Add,
		FlagSet: marketplaceconfig.FlagSet,
	}
}
