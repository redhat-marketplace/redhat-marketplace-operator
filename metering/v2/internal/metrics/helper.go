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
	"strings"

	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/marketplace/v1beta1"
	kbsm "k8s.io/kube-state-metrics/v2/pkg/metric"
	kbsmg "k8s.io/kube-state-metrics/v2/pkg/metric_generator"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("meteric")

type FamilyGenerator struct {
	GenerateMeterFunc func(interface{}, []*marketplacev1beta1.MeterDefinition) *kbsm.Family
	kbsmg.FamilyGenerator
}

func (g *FamilyGenerator) generateHeader() string {
	header := strings.Builder{}
	header.WriteString("# HELP ")
	header.WriteString(g.Name)
	header.WriteByte(' ')
	header.WriteString(g.Help)
	header.WriteByte('\n')
	header.WriteString("# TYPE ")
	header.WriteString(g.Name)
	header.WriteByte(' ')
	header.WriteString(string(g.Type))

	return header.String()
}

func GetMeterDefLabelsKeys(mdef *marketplacev1beta1.MeterDefinition) ([]string, []string) {
	return []string{"meter_def_name", "meter_def_namespace", "meter_def_group", "meter_kind"},
		[]string{mdef.Name, mdef.Namespace, mdef.Spec.Group, mdef.Spec.Kind}
}

func GetAllMeterLabelsKeys(mdefs []*marketplacev1beta1.MeterDefinition) ([]string, []string) {
	allMdefLabelKeys, allMdefLabelValues := []string{}, []string{}
	for _, meterDef := range mdefs {
		mdefLabelKeys, mdefLabelValues := GetMeterDefLabelsKeys(meterDef)
		allMdefLabelKeys = append(allMdefLabelKeys, mdefLabelKeys...)
		allMdefLabelValues = append(allMdefLabelValues, mdefLabelValues...)
	}

	return allMdefLabelKeys, allMdefLabelValues
}

func MapMeterDefinitions(metrics []*kbsm.Metric, mdefs []*marketplacev1beta1.MeterDefinition) []*kbsm.Metric {
	if len(mdefs) == 0 {
		return metrics
	}

	newMeters := make([]*kbsm.Metric, 0, len(mdefs))

	for _, m := range metrics {
		for _, mdef := range mdefs {
			mdefLabelKeys, mdefLabelValues := GetMeterDefLabelsKeys(mdef)

			newMeters = append(newMeters, &kbsm.Metric{
				Value:       m.Value,
				LabelKeys:   append(m.LabelKeys, mdefLabelKeys...),
				LabelValues: append(m.LabelValues, mdefLabelValues...),
			})
		}
	}

	return newMeters
}
