// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package report

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/reporter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

func initializeMarketplaceReporter(
	ctx context.Context,
	reportName reporter.ReportName,
	config reporter.Config,
	stopCh <-chan struct{},
) (*reporter.MarketplaceReporter, error) {
	panic(wire.Build(
		provideOptions,
		controller.SchemeDefinitions,
		reporter.ReporterSet,
		managers.ProvideManagerSet,
		managers.ProvideStartedClient,
		reconcileutils.CommandRunnerProviderSet,
		wire.InterfaceValue(new(logr.Logger), log),
		getMarketplaceReport,
		getPrometheusService,
		getMeterDefinitions,
	))
}
