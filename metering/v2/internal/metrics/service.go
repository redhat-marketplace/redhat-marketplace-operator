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
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		GenerateMeterFunc: wrapObject(descServiceLabelsDefaultLabels, func(obj client.Object, mdefs []*marketplacev1beta1.MeterDefinition) *kbsm.Family {
			s := obj.(*v1.Service)

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
