package controller

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/meterbase"
)

type MeterbaseController ControllerDefinition

func ProvideMeterbaseController() *MeterbaseController {
	return &MeterbaseController{
		Add:     meterbase.Add,
		FlagSet: meterbase.FlagSet,
	}
}
