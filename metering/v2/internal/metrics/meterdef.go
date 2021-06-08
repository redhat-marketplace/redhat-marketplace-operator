// Copyright 2020 IBM Corp.
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

package metrics

import (
	"reflect"

	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
)

var (
	descMeterDefinitionLabelsDefaultLabels = []string{}
)

var meterDefinitionMetricsFamilies = []FamilyGenerator{
	{
		FamilyGenerator: kbsm.FamilyGenerator{
			Name: "meterdef_metric_label_info",
			Type: kbsm.Gauge,
			Help: "Metering info for meterDefinition",
		},
		GenerateMeterFunc: wrapMeterDefinitionFunc(func(meterDefinition *marketplacev1beta1.MeterDefinition, meterDefinitions []*marketplacev1beta1.MeterDefinition) *kbsm.Family {
			metrics := []*kbsm.Metric{}
			allLabels := meterDefinition.ToPrometheusLabels()

			for _, labels := range allLabels {
				keys := []string{}
				values := []string{}

				labelMap, err := labels.ToLabels()
				if err != nil {
					log.Error(err, "label map conversion failed")
					continue
				}

				for key, value := range labelMap {
					keys = append(keys, key)
					values = append(values, value)
				}

				metrics = append(metrics, &kbsm.Metric{
					LabelKeys:   keys,
					LabelValues: values,
					Value:       1,
				})
			}

			return &kbsm.Family{
				Metrics: metrics,
			}
		}),
	},
}

// wrapMeterDefinitionFunc is a helper function for generating meterDefinition-based metrics
func wrapMeterDefinitionFunc(f func(*marketplacev1beta1.MeterDefinition, []*marketplacev1beta1.MeterDefinition) *kbsm.Family) func(obj interface{}, mdefs []*marketplacev1beta1.MeterDefinition) *kbsm.Family {
	return func(obj interface{}, meterDefinitions []*marketplacev1beta1.MeterDefinition) *kbsm.Family {
		meterDefinition := obj.(*marketplacev1beta1.MeterDefinition)

		metricFamily := f(meterDefinition, []*marketplacev1beta1.MeterDefinition{})
		metricFamily.Metrics = MapMeterDefinitions(metricFamily.Metrics, meterDefinitions)

		return metricFamily
	}
}

func ProvideMeterDefPrometheusData() *PrometheusDataMap {
	metricFamilies := meterDefinitionMetricsFamilies
	composedMetricGenFuncs := ComposeMetricGenFuncs(metricFamilies)
	familyHeaders := ExtractMetricFamilyHeaders(metricFamilies)

	return &PrometheusDataMap{
		expectedType:        reflect.TypeOf(&marketplacev1beta1.MeterDefinition{}),
		headers:             familyHeaders,
		metrics:             make(map[string][][]byte),
		generateMetricsFunc: composedMetricGenFuncs,
	}
}
