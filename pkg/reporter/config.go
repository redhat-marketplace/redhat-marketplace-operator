package reporter

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"k8s.io/apimachinery/pkg/types"
)

type Name types.NamespacedName

type MarketplaceReporterConfig struct {
	Name            types.NamespacedName
	Schemes         []*controller.SchemeDefinition
	WatchNamespace  string
	OutputDirectory string
}

func NewMarketplaceReporterConfig(
	reportName Name,
	schemes []*controller.SchemeDefinition,
) (*MarketplaceReporterConfig, error) {
	return &MarketplaceReporterConfig{
		Name:           types.NamespacedName(reportName),
		Schemes:        schemes,
		WatchNamespace: "",
	}, nil
}
