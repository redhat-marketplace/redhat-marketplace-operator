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
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
)

var (
	descPodLabelsDefaultLabels = []string{"namespace", "pod"}
)

var podMetricsFamilies = []FamilyGenerator{
	{
		FamilyGenerator: kbsm.FamilyGenerator{
			Name: "meterdef_pod_info",
			Type: kbsm.Gauge,
			Help: "Metering info for pod",
		},
		GenerateMeterFunc: wrapPodFunc(func(pod *corev1.Pod, meterDefinitions []*marketplacev1alpha1.MeterDefinition) *kbsm.Family {
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

// wrapPodFunc is a helper function for generating pod-based metrics
func wrapPodFunc(f func(*v1.Pod, []*marketplacev1alpha1.MeterDefinition) *kbsm.Family) func(obj interface{}, meterDefinitions []*marketplacev1alpha1.MeterDefinition) *kbsm.Family {
	return func(obj interface{}, meterDefinitions []*marketplacev1alpha1.MeterDefinition) *kbsm.Family {
		pod := obj.(*v1.Pod)

		metricFamily := f(pod, meterDefinitions)

		for _, m := range metricFamily.Metrics {
			m.LabelKeys = append(descPodLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{pod.Namespace, pod.Name}, m.LabelValues...)
		}

		metricFamily.Metrics = MapMeterDefinitions(metricFamily.Metrics, meterDefinitions)

		return metricFamily
	}
}
