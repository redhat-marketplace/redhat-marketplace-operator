package reporter

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"k8s.io/apimachinery/pkg/types"
)

type ReporterName types.NamespacedName

type MarketplaceReporterConfig struct {
	Name           types.NamespacedName
	Schemes        []*managers.SchemeDefinition
	WatchNamespace string
}

func NewMarketplaceReporterConfig(
	reportName ReporterName,
	schemes []*managers.SchemeDefinition,
) (*MarketplaceReporterConfig, error) {
	return &MarketplaceReporterConfig{
		Name:           (types.NamespacedName)(reportName),
		Schemes:        schemes,
		WatchNamespace: "",
	}, nil
}
