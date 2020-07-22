package controller

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/meterreport"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type MeterReportController struct {
	*baseDefinition
}

func ProvideMeterReportController(
	commandRunner reconcileutils.ClientCommandRunnerProvider,
	cfg *config.OperatorConfig,
) *MeterReportController {
	return &MeterReportController{
		baseDefinition: &baseDefinition{
			AddFunc: func(mgr manager.Manager) error {
				return meterreport.Add(mgr, commandRunner, cfg)
			},
			FlagSetFunc: func() *pflag.FlagSet { return nil },
		},
	}
}
