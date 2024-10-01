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

package v3alpha1

import (
	"fmt"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/cespare/xxhash"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	marketplacecommon "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"

	mapstructure "github.com/go-viper/mapstructure/v2"
)

type MarketplaceReportDataBuilder struct {
	id                              string
	values                          []*marketplacecommon.MeterDefPrometheusLabelsTemplated
	reportStart, reportEnd          common.Time
	resourceName, resourceNamespace string
	accountID, clusterID            string
	kvMap                           map[string]interface{}
	nsLabels                        map[string]map[string]string
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

func (d *MarketplaceReportDataBuilder) SetNamespaceLabels(nsLabels map[string]map[string]string) {
	d.nsLabels = nsLabels
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

	data := &MarketplaceReportData{}

	// Decode MeterDef LabelMap to top level Additional Properties
	mapstructure.Decode(data, meterDef.LabelMap)

	// 	Additional Properties from MeterDef
	data.Group = meterDef.MeterGroup
	data.Kind = meterDef.MeterKind

	// Usage event properties
	data.IntervalStart = meterDef.IntervalStart.UTC().Sub(time.Unix(0, 0)).Milliseconds()
	data.IntervalEnd = meterDef.IntervalEnd.UTC().Sub(time.Unix(0, 0)).Milliseconds()
	data.AccountID = d.accountID
	data.MeasuredUsage = make([]MeasuredUsage, 0, len(d.values))

	/*
		Source                         string `json:"source,omitempty"`
		SourceSaas                     string `json:"sourceSaas,omitempty"`
		AccountIdSaas                  string `json:"accountIdSaas,omitempty"`
		SubscriptionIdSaas             string `json:"subscriptionIdSaas,omitempty"`
		ProductType                    string `json:"productType,omitempty"`
		LicensePartNumber              string `json:"licensePartNumber,omitempty"`
		ProductId                      string `json:"productId,omitempty"`
		SapEntitlementLine             string `json:"sapEntitlementLine,omitempty"`
		ProductName                    string `json:"productName,omitempty"`
		ParentProductId                string `json:"parentProductId,omitempty"`
		ParentProductName              string `json:"parentProductName,omitempty"`
		ParentProductMetricId          string `json:"parentProductMetricId,omitempty"`
		TopLevelProductId              string `json:"topLevelProductId,omitempty"`
		TopLevelProductName            string `json:"topLevelProductName,omitempty"`
		TopLevelProductMetricId        string `json:"topLevelProductMetricId,omitempty"`
		DswOfferAccountingSystemCode   string `json:"dswOfferAccountingSystemCode,omitempty"`
		DswSubscriptionAgreementNumber string `json:"dswSubscriptionAgreementNumber,omitempty"`
		SsmSubscriptionId              string `json:"ssmSubscriptionId,omitempty"`
		ICN                            string `json:"ICN,omitempty"`
		Group                          string `json:"group,omitempty"`
		GroupName                      string `json:"groupName,omitempty"`
		Kind                           string `json:"kind,omitempty"`
	*/

	// Measured Usage Slice

	for _, meterDef := range d.values {
		if meterDef.Hash() != d.id {
			return nil, ErrValueHashAreDifferent
		}

		value, err := strconv.ParseFloat(meterDef.Value, 64)
		if err != nil {
			return nil, ErrNotFloat64
		}

		measuredUsage := MeasuredUsage{}

		// Decode MeterDef LabelMap to usage level Additional Properties
		mapstructure.Decode(measuredUsage, meterDef.LabelMap)

		// Additional Properties

		// Namespace Labels
		namespacesLabels := []NamespaceLabels{}
		for ns, labels := range d.nsLabels {
			namespacesLabels = append(namespacesLabels, NamespaceLabels{Name: ns, Labels: labels})
		}
		measuredUsage.NamespacesLabels = namespacesLabels

		measuredUsage.ClusterId = d.clusterID

		switch meterDef.MetricType {
		case marketplacecommon.MetricTypeEmpty:
			// grandfather old meterdefs into license
			measuredUsage.MetricType = marketplacecommon.MetricTypeLicense.String()
		case marketplacecommon.MetricTypeBillable:
			fallthrough
		case marketplacecommon.MetricTypeAdoption:
			fallthrough
		case marketplacecommon.MetricTypeInfrastructure:
			fallthrough
		case marketplacecommon.MetricTypeLicense:
			measuredUsage.MetricType = meterDef.MetricType.String()
		default:
			return nil, errors.New("metricType is an unknown type: " + meterDef.MetricType.String())
		}

		// Measured Usage
		measuredUsage.MetricID = meterDef.Label
		measuredUsage.Value = value

		// --- Additional Properties ---

		/*
			MetricType             string      `json:"metricType,omitempty"`
			MetricAggregationType  string      `json:"metricAggregationType,omitempty"`
			MeasuredMetricId       string      `json:"measuredMetricId,omitempty"`
			ProductConversionRatio string      `json:"productConversionRatio,omitempty"`
			MeasuredValue          string      `json:"measuredValue,omitempty"`
			ClusterId              string      `json:"clusterId,omitempty"`
			Hostname               string      `json:"hostname,omitempty"`
			Namespace              []Namespace `json:"namespace,omitempty"`
			Pod                    string      `json:"pod,omitempty"`
			PlatformId             string      `json:"platformId,omitempty"`
			Meter_def_namespace    string      `json:"meter_def_namespace,omitempty"`
			Crn                    string      `json:"crn,omitempty"`
			IsViewable             string      `json:"isViewable,omitempty"`
			CalculateSummary       string      `json:"calculateSummary,omitempty"`
		*/

		data.MeasuredUsage = append(data.MeasuredUsage, measuredUsage)
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

	data.EventID = fmt.Sprintf("%x", hash.Sum64())

	return data, nil
}
