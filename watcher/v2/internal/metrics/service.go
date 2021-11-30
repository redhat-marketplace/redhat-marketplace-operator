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
	v1 "k8s.io/api/core/v1"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
)

var (
	descServiceLabelsDefaultLabels = []string{"namespace", "service"}
)

var serviceMetricsFamilies = []FamilyGenerator{
	{
		FamilyGenerator: kbsm.FamilyGenerator{
			Name: "meterdef_service_info",
			Type: kbsm.Gauge,
			Help: "Info about the service for servicemonitor",
		},
		GenerateMeterFunc: wrapServiceFunc(func(s *v1.Service, mdefs []marketplacev1beta1.MeterDefinition) *kbsm.Family {
			// kube-state-metric labels
			clusterIP := s.Spec.ClusterIP
			externalName := s.Spec.ExternalName
			loadBalancerIP := s.Spec.LoadBalancerIP

			m := kbsm.Metric{
				Value:       1,
				LabelKeys:   []string{"cluster_ip", "external_name", "load_balancer_ip"},
				LabelValues: []string{clusterIP, externalName, loadBalancerIP},
			}
			return &kbsm.Family{
				Metrics: []*kbsm.Metric{&m},
			}
		}),
	},
}

// wrapServiceFunc is a helper function for generating service-based metrics
func wrapServiceFunc(f func(*v1.Service, []marketplacev1beta1.MeterDefinition) *kbsm.Family) func(obj interface{}, meterDefinitions []marketplacev1beta1.MeterDefinition) *kbsm.Family {
	return func(obj interface{}, meterDefinitions []marketplacev1beta1.MeterDefinition) *kbsm.Family {
		svc := obj.(*v1.Service)

		metricFamily := f(svc, meterDefinitions)

		for _, m := range metricFamily.Metrics {
			m.LabelKeys = append(descServiceLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{svc.Namespace, svc.Name}, m.LabelValues...)
		}

		metricFamily.Metrics = MapMeterDefinitions(metricFamily.Metrics, meterDefinitions)

		return metricFamily
	}
}

func ProvideServicePrometheusData() *PrometheusDataMap {
	metricFamilies := serviceMetricsFamilies
	composedMetricGenFuncs := ComposeMetricGenFuncs(metricFamilies)
	familyHeaders := ExtractMetricFamilyHeaders(metricFamilies)

	return &PrometheusDataMap{
		expectedType:        reflect.TypeOf(&v1.Service{}),
		headers:             familyHeaders,
		metrics:             make(map[string][][]byte),
		generateMetricsFunc: composedMetricGenFuncs,
	}
}
