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

package metric_generator

import (
	v1 "k8s.io/api/core/v1"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
)

var (
	descPodLabelsDefaultLabels     = []string{"namespace", "pod"}
	descServiceLabelsDefaultLabels = []string{"namespace", "service"}
)

// wrapPodFunc is a helper function for generating pod-based metrics
func wrapPodFunc(f func(*v1.Pod) *kbsm.Family) func(interface{}) *kbsm.Family {
	return func(obj interface{}) *kbsm.Family {
		pod := obj.(*v1.Pod)

		metricFamily := f(pod)

		for _, m := range metricFamily.Metrics {
			m.LabelKeys = append(descPodLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{pod.Namespace, pod.Name}, m.LabelValues...)
		}

		return metricFamily
	}
}

// wrapServiceFunc is a helper function for generating service-based metrics
func wrapServiceFunc(f func(*v1.Service) *kbsm.Family) func(interface{}) *kbsm.Family {
	return func(obj interface{}) *kbsm.Family {
		svc := obj.(*v1.Service)

		metricFamily := f(svc)

		for _, m := range metricFamily.Metrics {
			m.LabelKeys = append(descServiceLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{svc.Namespace, svc.Name}, m.LabelValues...)
		}

		return metricFamily
	}
}
