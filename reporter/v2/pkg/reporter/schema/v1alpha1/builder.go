package v1alpha1

import (
	"fmt"

	"emperror.dev/errors"
	"github.com/cespare/xxhash"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	marketplacecommon "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
)

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
	meterDef *marketplacecommon.MeterDefPrometheusLabelsTemplated,
) *MarketplaceReportDataBuilder {
	if d.values == nil {
		d.values = []*marketplacecommon.MeterDefPrometheusLabelsTemplated{}
	}

	d.values = append(d.values, meterDef)

	return d
}

func (d *MarketplaceReportDataBuilder) SetReportInterval(start, end common.Time) *MarketplaceReportDataBuilder {
	d.reportStart = start
	d.reportEnd = end
	return d
}

const (
	ErrNoValuesSet                  = errors.Sentinel("no values set")
	ErrValueHashAreDifferent        = errors.Sentinel("value hashes are different")
	ErrDuplicateLabel               = errors.Sentinel("duplicate label")
	ErrAdditionalLabelsAreDifferent = errors.Sentinel("different additional label values")
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
		IntervalStart:     common.Time(meterDef.IntervalStart),
		IntervalEnd:       common.Time(meterDef.IntervalEnd),
		ResourceNamespace: meterDef.ResourceNamespace,
		ResourceName:      meterDef.ResourceName,
		Label:             meterDef.WorkloadName,
		Unit:              meterDef.Unit,
		AdditionalLabels:  make(map[string]interface{}),
		Metrics:           make(map[string]interface{}),
		MetricsExtended:   make([]MarketplaceMetric, 0, len(d.values)),
    RecordSummary: &MarketplaceReportRecordSummary{
		TotalMetricCount: len(d.values),
	},
	}

	if meterDef.MeterDescription != "" {
		key.AdditionalLabels["display_description"] = meterDef.MeterDescription
	}

	if meterDef.DisplayName != "" {
		key.AdditionalLabels["display_name"] = meterDef.DisplayName
	}

	allLabels := []map[string]interface{}{}
	dupeKeys := map[string]interface{}{}

	for _, meterDef := range d.values {
		if meterDef.Hash() != d.id {
			return nil, ErrValueHashAreDifferent
		}

		if _, ok := key.Metrics[meterDef.Label]; !ok {
			key.Metrics[meterDef.Label] = meterDef.Value
		}

		key.MetricsExtended = append(key.MetricsExtended,
			MarketplaceMetric{
				Label:   meterDef.Label,
				Value:  meterDef.Value,
			},
		)

		// Add the additional keys
		for k, value := range meterDef.LabelMap {
			if _, ok := dupeKeys[k]; ok {
				continue
			}

			v, ok := key.AdditionalLabels[k]

			if ok && v != value {
				dupeKeys[k] = nil
				delete(key.AdditionalLabels, k)
			} else if !ok {
				key.AdditionalLabels[k] = value
			}
		}
	}

	if len(dupeKeys) != 0 {
		for i, meterDef := range d.values {
			nonUniqueLabels := map[string]interface{}{}
			for duped := range dupeKeys {
				nonUniqueLabels[duped] = meterDef.LabelMap[duped]
			}

			key.MetricsExtended[i].Labels = nonUniqueLabels
			allLabels = append(allLabels, nonUniqueLabels)
		}
	}

	hash := xxhash.New()
	hash.Write([]byte(d.clusterID))
	hash.Write([]byte(d.id))
	key.MetricID = fmt.Sprintf("%x", hash.Sum64())

	return key, nil
}
