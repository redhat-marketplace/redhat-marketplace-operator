// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package report

import (
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/reporter"
)

func initializeMarketplaceReporter(reportName reporter.ReporterName) (*reporter.MarketplaceReporter, error){
	wire.Build(MarketplaceReporterSet)
	return &reporter.MarketplaceReporter{}, nil
}
