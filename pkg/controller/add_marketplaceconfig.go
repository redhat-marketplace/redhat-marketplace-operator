package controller

import (
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller/marketplaceconfig"
)

type MarketplaceController ControllerDefinition

func ProvideMarketplaceController() *MarketplaceController {
	return &MarketplaceController{
		Add:     marketplaceconfig.Add,
		FlagSet: marketplaceconfig.FlagSet,
	}
}
