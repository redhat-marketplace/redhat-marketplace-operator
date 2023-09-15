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
	descPersistentVolumeClaimLabelsDefaultLabels = []string{"namespace", "persistentvolumeclaim"}
)

var pvcMetricsFamilies = []FamilyGenerator{
	{
		FamilyGenerator: kbsmg.FamilyGenerator{
			Name: "meterdef_persistentvolumeclaim_info",
			Type: kbsm.Gauge,
			Help: "Metering info for persistentvolumeclaim",
		},
		GenerateMeterFunc: wrapObject(descPersistentVolumeClaimLabelsDefaultLabels, func(obj client.Object, meterDefinitions []*marketplacev1beta1.MeterDefinition) *kbsm.Family {
			pvc := obj.(*corev1.PersistentVolumeClaim)

			metrics := []*kbsm.Metric{}

			phase := pvc.Status.Phase

			metrics = append(metrics, &kbsm.Metric{
				LabelKeys:   []string{"phase"},
				LabelValues: []string{string(phase)},
				Value:       1,
			})

			return &kbsm.Family{
				Metrics: metrics,
			}
		}),
	},
}

func ProvidePersistentVolumeClaimPrometheusData() *PrometheusDataMap {
	metricFamilies := pvcMetricsFamilies
	composedMetricGenFuncs := ComposeMetricGenFuncs(metricFamilies)
	familyHeaders := ExtractMetricFamilyHeaders(metricFamilies)

	return &PrometheusDataMap{
		expectedType:        reflect.TypeOf(&corev1.PersistentVolumeClaim{}),
		headers:             familyHeaders,
		metrics:             make(map[string][][]byte),
		generateMetricsFunc: composedMetricGenFuncs,
	}
}
