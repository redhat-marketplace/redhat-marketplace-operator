package metric_generator

import (
	v1 "k8s.io/api/core/v1"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
)

var (
	descServiceLabelsDefaultLabels = []string{"namespace", "service"}
)

var serviceMetricsFamilies = []kbsm.FamilyGenerator{
	{
		Name: "meterdef_service_info",
		Type: kbsm.Gauge,
		Help: "Info about the service for servicemonitor",
		GenerateFunc: wrapServiceFunc(func(s *v1.Service) *kbsm.Family {
			meterDefDomain := s.ObjectMeta.Annotations["meter_def_domain"]
			meterDefKind := s.ObjectMeta.Annotations["meter_def_kind"]
			meterDefVersion := s.ObjectMeta.Annotations["meter_def_version"]

			// kube-state-metric labels
			clusterIP := s.Spec.ClusterIP
			externalName := s.Spec.ExternalName
			loadBalancerIP := s.Spec.LoadBalancerIP

			m := kbsm.Metric{
				LabelKeys:   []string{"meter_def_kind", "meter_def_version", "meter_def_domain", "cluster_ip", "external_name", "load_balancer_ip"},
				LabelValues: []string{meterDefKind, meterDefVersion, meterDefDomain, clusterIP, externalName, loadBalancerIP},
			}
			return &kbsm.Family{
				Metrics: []*kbsm.Metric{&m},
			}
		}),
	},
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
