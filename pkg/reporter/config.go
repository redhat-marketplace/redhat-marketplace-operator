package reporter

import (
	"github.com/google/wire"
	"github.com/gotidy/ptr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Name types.NamespacedName
type PrometheusService *corev1.Service
type ReportOutputDir string

// Top level config
type Config struct {
	OutputDirectory string
}

type reporterConfig struct {
	OutputDirectory string
	MetricsPerFile  *int
	MaxRoutines     *int
	Retry           *int
}

func ProvideReporterConfig(
	reportConfig Config,
) *reporterConfig {
	cfg := &reporterConfig{
		OutputDirectory: reportConfig.OutputDirectory,
	}
	cfg.setDefaults()

	return cfg
}

const (
	defaultMetricsPerFile = 500
	defaultMaxRoutines    = 50
)

func (c *reporterConfig) setDefaults() {
	if c.MetricsPerFile == nil {
		c.MetricsPerFile = ptr.Int(defaultMetricsPerFile)
	}

	if c.MaxRoutines == nil {
		c.MaxRoutines = ptr.Int(defaultMaxRoutines)
	}

	if c.Retry == nil {
		c.Retry = ptr.Int(5)
	}
}

var ReporterSet = wire.NewSet(
	ProvideReporterConfig,
	NewMarketplaceReporter,
	NewRedHatInsightsUploader,
)
