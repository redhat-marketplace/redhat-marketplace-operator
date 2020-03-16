package controller

import (
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller/meterbase"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, meterbase.Add)
	flagSets = append(flagSets, meterbase.FlagSet())
}
