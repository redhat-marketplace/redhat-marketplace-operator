package metrics

import (
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/logger"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
)

var log = logger.NewLogger("meterics")

func GetMeterDefLabelsKeys(mdef *marketplacev1alpha1.MeterDefinition) ([]string, []string) {
	return []string{"meter_def_name", "meter_def_namespace", "meter_def_domain", "meter_def_kind", "meter_def_version"},
		[]string{mdef.Name, mdef.Namespace, mdef.Spec.Group, mdef.Spec.Kind, mdef.Spec.Version}
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
