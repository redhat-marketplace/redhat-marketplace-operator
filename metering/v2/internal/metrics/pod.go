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
	corev1 "k8s.io/api/core/v1"
	kbsm "k8s.io/kube-state-metrics/v2/pkg/metric"
	kbsmg "k8s.io/kube-state-metrics/v2/pkg/metric_generator"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	descPodLabelsDefaultLabels = []string{"namespace", "pod"}
)

var podMetricsFamilies = []FamilyGenerator{
	{
		FamilyGenerator: kbsmg.FamilyGenerator{
			Name: "meterdef_pod_info",
			Type: kbsm.Gauge,
			Help: "Metering info for pod",
		},
		GenerateMeterFunc: wrapObject(descPodLabelsDefaultLabels, func(obj client.Object, meterDefinitions []*marketplacev1beta1.MeterDefinition) *kbsm.Family {
			pod := obj.(*corev1.Pod)
			metrics := []*kbsm.Metric{}

			podUID := string(pod.UID)
			priorityClass := pod.Spec.PriorityClassName

			metrics = append(metrics, &kbsm.Metric{
				LabelKeys:   []string{"pod_uid", "priority_class"},
				LabelValues: []string{podUID, priorityClass},
				Value:       1,
			})

			return &kbsm.Family{
				Metrics: metrics,
			}
		}),
	},
}

// wrapObject is a helper function for generating the base metrics for all client objects (pod, service, pvc)
func wrapObject(labels []string, f func(client.Object, []*marketplacev1beta1.MeterDefinition) *kbsm.Family) func(obj interface{}, meterDefinitions []*marketplacev1beta1.MeterDefinition) *kbsm.Family {
	return func(in interface{}, meterDefinitions []*marketplacev1beta1.MeterDefinition) *kbsm.Family {
		obj := in.(client.Object)
		metricFamily := f(obj, meterDefinitions)

		for _, m := range metricFamily.Metrics {
			m.LabelKeys = append(labels, m.LabelKeys...)
			m.LabelValues = append([]string{obj.GetNamespace(), obj.GetName()}, m.LabelValues...)
		}

		metricFamily.Metrics = MapMeterDefinitions(metricFamily.Metrics, meterDefinitions)

		return metricFamily
	}
}

func ProvidePodPrometheusData() *PrometheusDataMap {
	metricFamilies := podMetricsFamilies
	composedMetricGenFuncs := ComposeMetricGenFuncs(metricFamilies)
	familyHeaders := ExtractMetricFamilyHeaders(metricFamilies)

	return &PrometheusDataMap{
		expectedType:        reflect.TypeOf(&corev1.Pod{}),
		headers:             familyHeaders,
		metrics:             make(map[string][][]byte),
		generateMetricsFunc: composedMetricGenFuncs,
	}
}
