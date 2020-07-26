// +build wireinject

package reporter

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

func NewTask(
	ctx context.Context,
	reportName ReportName,
	config *Config,
) (*Task, error) {
	panic(wire.Build(
		managers.ProvideCachedClientSet,
		wire.Struct(new(Task), "*"),
		getClientOptions,
		controller.SchemeDefinitions,
	))
}

func NewReporter(
	task *Task,
) (*MarketplaceReporter, error) {
	panic(wire.Build(
		wire.FieldsOf(new(*Task),
			"ReportName", "K8SClient", "Ctx", "Config", "K8SScheme"),
		reconcileutils.CommandRunnerProviderSet,
		wire.InterfaceValue(new(logr.Logger), logger),
		getMarketplaceReport,
		getPrometheusService,
		getMeterDefinitions,
		ReporterSet,
	))
}
