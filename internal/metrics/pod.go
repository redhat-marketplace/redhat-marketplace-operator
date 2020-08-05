package metrics

import (
	"context"
	"strings"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/meter_definition"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
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

type PodMeterDefFetcher struct {
	cc                   ClientCommandRunner
	meterDefinitionStore *meter_definition.MeterDefinitionStore
}

func (p *PodMeterDefFetcher) GetMeterDefinitions(obj interface{}) ([]*marketplacev1alpha1.MeterDefinition, error) {
	results := []*marketplacev1alpha1.MeterDefinition{}
	pod, ok := obj.(*corev1.Pod)

	if !ok {
		return results, nil
	}

	refs := p.meterDefinitionStore.GetMeterDefinitionRefs(pod.UID)

	for _, ref := range refs {
		meterDefinition := &marketplacev1alpha1.MeterDefinition{}

		result, _ := p.cc.Do(
			context.TODO(),
			GetAction(ref.MeterDef, meterDefinition),
		)

		if !result.Is(Continue) {
			if result.Is(Error) {
				log.Error(result, "failed to get owner")
				return results, result
			}
			return results, nil
		}

		results = append(results, meterDefinition)
	}

	return results, nil
}
