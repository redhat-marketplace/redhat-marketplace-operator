package reporter

import (
	"fmt"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	schemav1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/v1alpha1"
	schemav2alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/v2alpha1"
	writerv1 "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/writer/v1"
	writerv2 "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/writer/v2"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
)

func ProvideWriter(
	config *Config,
	MktConfig *marketplacev1alpha1.MarketplaceConfig,
	logger logr.Logger,
) (common.ReportWriter, error) {
	switch config.ReporterSchema {
	case "v1alpha1":
		return &writerv1.ReportWriter{MktConfig: MktConfig, Logger: logger}, nil
	case "v2alpha1":
		return &writerv2.ReportWriter{MktConfig: MktConfig, Logger: logger}, nil
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported reporterSchema: %s", config.ReporterSchema))
	}
}

func ProvideDataBuilder(
	config *Config,
	logger logr.Logger,
) (common.DataBuilder, error) {
	switch config.ReporterSchema {
	case "v1alpha1":
		return common.DataBuilderFunc(func() common.SchemaMetricBuilder {
			return &schemav1alpha1.MarketplaceReportDataBuilder{}
		}), nil
	case "v2alpha1":
		return common.DataBuilderFunc(func() common.SchemaMetricBuilder {
			return &schemav2alpha1.MarketplaceReportDataBuilder{}
		}), nil
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported reporterSchema: %s", config.ReporterSchema))
	}
}
