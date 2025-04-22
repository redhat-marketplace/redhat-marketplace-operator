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
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/cespare/xxhash"
	mapstructure "github.com/go-viper/mapstructure/v2"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	marketplacecommon "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = logf.Log.WithName("builder")

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

	data := &MarketplaceReportData{}

	// Decode MeterDef LabelMap to top level Additional Properties
	if err := mapstructure.Decode(&meterDef.LabelMap, data); err != nil {
		return nil, err
	}

	// 	Additional Properties from MeterDef
	data.Group = meterDef.MeterGroup
	data.Kind = meterDef.MeterKind

	// Usage event properties
	data.IntervalStart = meterDef.IntervalStart.UTC().Sub(time.Unix(0, 0)).Milliseconds()
	data.IntervalEnd = meterDef.IntervalEnd.UTC().Sub(time.Unix(0, 0)).Milliseconds()
	data.AccountID = d.accountID
	data.MeasuredUsage = make([]MeasuredUsage, 0, len(d.values))

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
		if err := mapstructure.Decode(&meterDef.LabelMap, &measuredUsage); err != nil {
			return nil, err
		}

		// Namespace Labels
		// schema accepts a stringified map
		namespacesLabels := []NamespaceLabels{}
		for ns, labels := range meterDef.NamespaceLabels {
			namespacesLabels = append(namespacesLabels, NamespaceLabels{Name: ns, Labels: labels})
		}
		namespacesLabelsBytes, err := json.Marshal(namespacesLabels)
		if err != nil {
			return nil, err
		}
		if len(namespacesLabelsBytes) != 0 {
			measuredUsage.NamespacesLabels = string(namespacesLabelsBytes)
		}

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
			// set as RawMessage for Marshal as the resources are already serialized
			kubernetesResources := []json.RawMessage{}
			for _, k8sR := range meterDef.K8sResources {
				b, ok := k8sR.([]byte)
				if ok {
					kubernetesResources = append(kubernetesResources, json.RawMessage(b))
				}
			}
			measuredUsage.KubernetesResources = kubernetesResources
			fallthrough
		case marketplacecommon.MetricTypeLicense:
			measuredUsage.MetricType = meterDef.MetricType.String()
		default:
			return nil, errors.New("metricType is an unknown type: " + meterDef.MetricType.String())
		}

		// Measured Usage
		measuredUsage.MetricID = meterDef.Label
		measuredUsage.Value = value

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
