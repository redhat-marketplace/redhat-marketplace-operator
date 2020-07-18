package reporter

import (
	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Name types.NamespacedName

type MarketplaceReporterConfig struct {
	PrometheusService *corev1.Service
	Schemes           []*controller.SchemeDefinition
	WatchNamespace    string
	OutputDirectory   string
	MetricsPerFile    *int
	MaxRoutines       *int
}

type ReportOutputDir string

func ProvideMarketplacReportConfig(
	prometheusService *corev1.Service,
	localSchemes controller.LocalSchemes,
	dir ReportOutputDir,
) *MarketplaceReporterConfig {
	cfg := &MarketplaceReporterConfig{
		OutputDirectory:   string(dir),
		PrometheusService: prometheusService,
		Schemes:           localSchemes,
	}
	cfg.setDefaults()

	return cfg
}

const (
	defaultMetricsPerFile = 500
	defaultMaxRoutines    = 50
)

func (c *MarketplaceReporterConfig) setDefaults() {
	if c.MetricsPerFile == nil {
		c.MetricsPerFile = ptr.Int(defaultMetricsPerFile)
	}

	if c.MaxRoutines == nil {
		c.MaxRoutines = ptr.Int(defaultMaxRoutines)
	}
}
