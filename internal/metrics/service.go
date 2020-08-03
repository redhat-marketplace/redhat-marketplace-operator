package metrics

import (
	"context"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/meter_definition"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
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
		GenerateMeterFunc: wrapServiceFunc(func(s *v1.Service, mdefs []*marketplacev1alpha1.MeterDefinition) *kbsm.Family {
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
func wrapServiceFunc(f func(*v1.Service, []*marketplacev1alpha1.MeterDefinition) *kbsm.Family) func(obj interface{}, meterDefinitions []*marketplacev1alpha1.MeterDefinition) *kbsm.Family {
	return func(obj interface{}, meterDefinitions []*marketplacev1alpha1.MeterDefinition) *kbsm.Family {
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

type ServiceMeterDefFetcher struct {
	cc ClientCommandRunner
	meterDefinitionStore *meter_definition.MeterDefinitionStore
}

func (p *ServiceMeterDefFetcher) GetMeterDefinitions(obj interface{}) ([]*marketplacev1alpha1.MeterDefinition, error) {
	results := []*marketplacev1alpha1.MeterDefinition{}
	service, ok := obj.(*v1.Service)

	if !ok {
		return results, nil
	}

	refs := p.meterDefinitionStore.GetMeterDefinitionRefs(service.UID)

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
