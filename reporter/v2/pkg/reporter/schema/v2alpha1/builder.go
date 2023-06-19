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

package v2alpha1

import (
	"fmt"
	"strconv"
	"time"

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
	ErrNotFloat64                   = errors.Sentinel("meterdefinition value not float64")
)

func (d *MarketplaceReportDataBuilder) Build() (interface{}, error) {
	if len(d.values) == 0 {
		return nil, ErrNoValuesSet
	}

	meterDef := d.values[0]
	d.id = meterDef.Hash()

	key := &MarketplaceReportData{
		IntervalStart:        meterDef.IntervalStart.UTC().Sub(time.Unix(0, 0)).Milliseconds(),
		IntervalEnd:          meterDef.IntervalEnd.UTC().Sub(time.Unix(0, 0)).Milliseconds(),
		AccountID:            d.accountID,
		AdditionalAttributes: make(map[string]interface{}),
		MeasuredUsage:        make([]MeasuredUsage, 0, len(d.values)),
	}

	if d.clusterID != "" {
		key.AdditionalAttributes["clusterId"] = d.clusterID
	}

	if meterDef.MeterGroup != "" {
		key.AdditionalAttributes["group"] = meterDef.MeterGroup
	}

	if meterDef.MeterKind != "" {
		key.AdditionalAttributes["kind"] = meterDef.MeterKind
	}

	if d.resourceNamespace != "" {
		key.AdditionalAttributes["namespace"] = d.resourceNamespace
	}

	switch meterDef.MetricType {
	case marketplacecommon.MetricTypeEmpty:
		// grandfather old meterdefs into license
		key.AdditionalAttributes["metricType"] = marketplacecommon.MetricTypeLicense.String()
	case marketplacecommon.MetricTypeBillable:
		fallthrough
	case marketplacecommon.MetricTypeAdoption:
		fallthrough
	case marketplacecommon.MetricTypeInfrastructure:
		fallthrough
	case marketplacecommon.MetricTypeLicense:
		key.AdditionalAttributes["metricType"] = meterDef.MetricType.String()
	default:
		return nil, errors.New("metricType is an unknown type: " + meterDef.MetricType.String())
	}

	/* Additional well known attributes */
	// key.AdditionalAttributes["hostname"]
	// key.AdditionalAttributes["source"]
	// key.AdditionalAttributes["metricType"]
	// key.AdditionalAttributes["manual"]
	// key.AdditionalAttributes["measuredValue"]
	// key.AdditionalAttributes["measuredMetricId"]
	// key.AdditionalAttributes["productId"]
	// key.AdditionalAttributes["productName"]
	// key.AdditionalAttributes["parentProductId"]
	// key.AdditionalAttributes["productType"]
	// key.AdditionalAttributes["parentProductName"]
	// key.AdditionalAttributes["productConversionRatio"]

	allLabels := []map[string]interface{}{}
	dupeKeys := map[string]interface{}{}

	for _, meterDef := range d.values {
		if meterDef.Hash() != d.id {
			return nil, ErrValueHashAreDifferent
		}

		value, err := strconv.ParseFloat(meterDef.Value, 64)
		if err != nil {
			return nil, ErrNotFloat64
		}

		measuredUsage := MeasuredUsage{
			MetricID:             meterDef.Label,
			Value:                value,
			AdditionalAttributes: make(map[string]interface{}),
		}

		// Add the additional keys
		for k, value := range meterDef.LabelMap {
			if _, ok := dupeKeys[k]; ok {
				continue
			}

			v, ok := key.AdditionalAttributes[k]

			if ok && v != value {
				dupeKeys[k] = nil
				delete(key.AdditionalAttributes, k)
			} else if !ok {
				key.AdditionalAttributes[k] = value
			}
		}

		key.MeasuredUsage = append(key.MeasuredUsage, measuredUsage)
	}

	if len(dupeKeys) != 0 {
		for i, meterDef := range d.values {
			nonUniqueLabels := map[string]interface{}{}
			for duped := range dupeKeys {
				nonUniqueLabels[duped] = meterDef.LabelMap[duped]
			}

			key.MeasuredUsage[i].AdditionalAttributes = nonUniqueLabels
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

	key.EventID = fmt.Sprintf("%x", hash.Sum64())

	return key, nil
}
