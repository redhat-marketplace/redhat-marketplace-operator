package metrics

import (
	"context"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
}

func (s *ServiceMeterDefFetcher) GetMeterDefinitions(obj interface{}) ([]*marketplacev1alpha1.MeterDefinition, error) {
	results := []*marketplacev1alpha1.MeterDefinition{}
	service, ok := obj.(*v1.Service)

	if !ok {
		return results, nil
	}

	owner := metav1.GetControllerOf(service)

	if owner == nil {
		return results, nil
	}

	ownerGVK := rhmclient.ObjRefToStr(owner.APIVersion, owner.Kind)

	meterDefinitions := &marketplacev1alpha1.MeterDefinitionList{}
	result, _ := s.cc.Do(
		context.TODO(),
		ListAction(meterDefinitions, client.MatchingField(rhmclient.MeterDefinitionGVK, ownerGVK)),
	)

	if !result.Is(Continue) {
		if result.Is(Error) {
			log.Error(result, "failed to get owner")
			return results, result
		}
		return results, nil
	}

	for i := 0; i < len(meterDefinitions.Items); i++ {
		results = append(results, &meterDefinitions.Items[i])
	}

	return results, nil
}
