// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/cespare/xxhash"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	marketplacecommon "github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/marketplace/common"
)

type MarketplaceReportDataBuilder struct {
	id                              string
	values                          []*marketplacecommon.MeterDefPrometheusLabelsTemplated
	reportStart, reportEnd          common.Time
	resourceName, resourceNamespace string
	accountID, clusterID            string
	kvMap                           map[string]interface{}
}

func (d *MarketplaceReportDataBuilder) SetClusterID(
	clusterID string,
) {
	d.clusterID = clusterID
}

func (d *MarketplaceReportDataBuilder) SetAccountID(
	accountID string,
) {
	d.accountID = accountID
}

func (d *MarketplaceReportDataBuilder) AddMeterDefinitionLabels(
	meterDef *marketplacecommon.MeterDefPrometheusLabelsTemplated,
) {
	if d.values == nil {
		d.values = []*marketplacecommon.MeterDefPrometheusLabelsTemplated{}
	}

	d.values = append(d.values, meterDef)
}

func (d *MarketplaceReportDataBuilder) SetReportInterval(start, end common.Time) {
	d.reportStart = start
	d.reportEnd = end
}

const (
	ErrNoValuesSet                  = errors.Sentinel("no values set")
	ErrValueHashAreDifferent        = errors.Sentinel("value hashes are different")
	ErrDuplicateLabel               = errors.Sentinel("duplicate label")
	ErrAdditionalLabelsAreDifferent = errors.Sentinel("different additional label values")
)

func (d *MarketplaceReportDataBuilder) Build() (interface{}, error) {
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
				Label: meterDef.Label,
				Value: meterDef.Value,
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
	hash.Write([]byte(meterDef.IntervalStart.UTC().Format(time.RFC3339)))
	hash.Write([]byte(meterDef.IntervalEnd.UTC().Format(time.RFC3339)))
	hash.Write([]byte(meterDef.MeterGroup))
	hash.Write([]byte(meterDef.MeterKind))
	hash.Write([]byte(meterDef.Metric))
	hash.Write([]byte(meterDef.ResourceNamespace))
	hash.Write([]byte(meterDef.ResourceName))
	hash.Write([]byte(meterDef.Unit))
	key.MetricID = fmt.Sprintf("%x", hash.Sum64())
	return key, nil
}
