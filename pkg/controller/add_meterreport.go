package controller

import (
	"github.com/spf13/pflag"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller/meterreport"
)

type MeterReportDefinition ControllerDefinition

func ProvideMeterReportDefinition() *MeterReportDefinition {
	return &MeterReportDefinition{
		Add:     meterreport.Add,
		FlagSet: func() *pflag.FlagSet { return nil },
	}
}
