package v1alpha1

import (
	"encoding/json"
	"fmt"

	"emperror.dev/errors"
	"github.com/cespare/xxhash"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	marketplacecommon "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
)

type MarketplaceReportSlice struct {
	ReportSliceID common.ReportSliceKey   `json:"report_slice_id"`
	Metadata      *SourceMetadata         `json:"metadata,omitempty"`
	Metrics       []MarketplaceReportData `json:"metrics"`
}

type MarketplaceReportData struct {
	MetricID          string                 `json:"metric_id"`
	ReportPeriodStart common.Time            `json:"report_period_start"`
	ReportPeriodEnd   common.Time            `json:"report_period_end"`
	IntervalStart     common.Time            `json:"interval_start"`
	IntervalEnd       common.Time            `json:"interval_end"`
	MeterDomain       string                 `json:"domain"`
	MeterKind         string                 `json:"kind"`
	MeterVersion      string                 `json:"version,omitempty"`
	Label             string                 `json:"workload,omitempty"`
	Namespace         string                 `json:"namespace,omitempty"`
	ResourceName      string                 `json:"resource_name,omitempty"`
	Unit              string                 `json:"unit,omitempty"`
	AdditionalLabels  map[string]interface{} `mapstructure:"additionalLabels"`
	Metrics           map[string]interface{} `mapstructure:"rhmUsageMetrics"`
}

type MarketplaceReportDataBuilder struct {
	id                              string
	values                          []*marketplacecommon.MeterDefPrometheusLabelsTemplated
	reportStart, reportEnd          common.Time
	resourceName, resourceNamespace string
	clusterID                       string
	kvMap                           map[string]interface{}
}

func (d *MarketplaceReportDataBuilder) SetClusterID(
	clusterID string,
) *MarketplaceReportDataBuilder {
	d.clusterID = clusterID
	return d
}

func (d *MarketplaceReportDataBuilder) AddMeterDefinitionLabels(
	meterDef *marketplacecommon.MeterDefPrometheusLabels,
) *MarketplaceReportDataBuilder {
	if d.values == nil {
		d.values = []*marketplacecommon.MeterDefPrometheusLabels{}
	}

	d.values = append(d.values, meterDef)

	return d
}

func (d *MarketplaceReportDataBuilder) SetReportInterval(start, end common.Time) *MarketplaceReportDataBuilder {
	d.reportStart = start
	d.reportEnd = end
	return d
}

func (d *MarketplaceReportDataBuilder) SetLabelMap(labelMap map[string]interface{}) *MarketplaceReportDataBuilder {
	d.kvMap = labelMap
	return d
}

const (
	ErrNoValuesSet           = errors.Sentinel("no values set")
	ErrValueHashAreDifferent = errors.Sentinel("value hashes are different")
)

func (d *MarketplaceReportDataBuilder) Build() (*MarketplaceReportData, error) {
	if len(d.values) == 0 {
		return nil, ErrNoValuesSet
	}

	meterDef := d.values[0]
	d.id = meterDef.Hash()


	key := &MarketplaceReportData{
		ReportPeriodStart: d.reportStart,
		ReportPeriodEnd:   d.reportEnd,
		MeterDomain:       meterDef.MeterGroup,
		MeterKind:         meterDef.MeterKind,
		IntervalStart:     meterDef.IntervalStart,
		IntervalEnd:       meterDef.IntervalEnd,
		Namespace:         d.resourceNamespace,
		ResourceName:      d.resourceName,
		Unit:              meterDef.Unit,
		Label:             meterDef.WorkloadName,
	}

	for _, meterDef := range d.values[1:] {
		if meterDef.Hash() != d.id {
			return nil, ErrValueHashAreDifferent
		}
	}

	hash := xxhash.New()
	hash.Write([]byte(d.clusterID))
	hash.Write([]byte(d.id))

	key.MetricID = fmt.Sprintf("%x", hash.Sum64())

	key.AdditionalLabels = d.kvMap
	key.Metrics[mKey] = value

	return key, nil
}
