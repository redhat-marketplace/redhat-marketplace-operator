package controller

import (
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller/meterbase"
)

type MeterbaseController ControllerDefinition

func ProvideMeterbaseController() *MeterbaseController {
	return &MeterbaseController{
		Add:     meterbase.Add,
		FlagSet: meterbase.FlagSet,
	}
}
