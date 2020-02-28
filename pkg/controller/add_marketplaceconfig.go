package controller

import (
	"github.ibm.com/symposium/marketplace-operator/pkg/controller/marketplaceconfig"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, marketplaceconfig.Add)
	flagSets = append(flagSets, marketplaceconfig.FlagSet())
}
