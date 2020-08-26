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
	"context"
	"strings"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/meter_definition"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("meteric")

type FamilyGenerator struct {
	GenerateMeterFunc func(interface{}, []*marketplacev1alpha1.MeterDefinition) *kbsm.Family
	kbsm.FamilyGenerator
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

func GetMeterDefLabelsKeys(mdef *marketplacev1alpha1.MeterDefinition) ([]string, []string) {
	return []string{"meter_def_name", "meter_def_namespace", "meter_def_domain", "meter_def_kind"},
		[]string{mdef.Name, mdef.Namespace, mdef.Spec.Group, mdef.Spec.Kind}
}

func GetAllMeterLabelsKeys(mdefs []*marketplacev1alpha1.MeterDefinition) ([]string, []string) {
	allMdefLabelKeys, allMdefLabelValues := []string{}, []string{}
	for _, meterDef := range mdefs {
		mdefLabelKeys, mdefLabelValues := GetMeterDefLabelsKeys(meterDef)
		allMdefLabelKeys = append(allMdefLabelKeys, mdefLabelKeys...)
		allMdefLabelValues = append(allMdefLabelValues, mdefLabelValues...)
	}

	return allMdefLabelKeys, allMdefLabelValues
}

func MapMeterDefinitions(metrics []*kbsm.Metric, mdefs []*marketplacev1alpha1.MeterDefinition) []*kbsm.Metric {
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

type MeterDefFetcher struct {
	cc                   reconcileutils.ClientCommandRunner
	meterDefinitionStore *meter_definition.MeterDefinitionStore
}

func (p *MeterDefFetcher) GetMeterDefinitions(obj interface{}) ([]*marketplacev1alpha1.MeterDefinition, error) {
	results := []*marketplacev1alpha1.MeterDefinition{}
	metaobj, err := meta.Accessor(obj)

	if err != nil {
		return results, err
	}

	return p.getMeterDefs(metaobj.GetUID())
}

func (p *MeterDefFetcher) getMeterDefs(
	uid types.UID,
) ([]*marketplacev1alpha1.MeterDefinition, error) {
	results := []*marketplacev1alpha1.MeterDefinition{}
	refs := p.meterDefinitionStore.GetMeterDefinitionRefs(uid)

	for _, ref := range refs {
		meterDefinition := &marketplacev1alpha1.MeterDefinition{}
		err := p.getMeterDef(ref.MeterDef, meterDefinition)

		if err != nil {
			return results, err
		}

		results = append(results, meterDefinition)
	}

	return results, nil

}

func (p *MeterDefFetcher) getMeterDef(
	name types.NamespacedName,
	mdef *marketplacev1alpha1.MeterDefinition,
) error {
	result, _ := p.cc.Do(
		context.TODO(),
		reconcileutils.GetAction(name, mdef),
	)

	if !result.Is(reconcileutils.Continue) {
		if result.Is(reconcileutils.Error) {
			log.Error(result, "failed to get owner")
			return result
		}
		return result
	}

	return nil
}
