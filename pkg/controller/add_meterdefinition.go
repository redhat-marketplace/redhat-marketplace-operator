package controller

import (
	"github.com/spf13/pflag"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/meterdefinition"
)

type MeterDefinitionController ControllerDefinition

func ProvideMeterDefinitionController() *MeterDefinitionController {
	return &MeterDefinitionController{
		Add:     meterdefinition.Add,
		FlagSet: func() *pflag.FlagSet { return nil },
	}
}
