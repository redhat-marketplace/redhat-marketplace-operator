package metric_generator

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
)

var (
	descPodLabelsDefaultLabels = []string{"namespace", "pod"}
)

var podMetricsFamilies = []kbsm.FamilyGenerator{
	{
		Name: "meterdef_pod_info",
		Type: kbsm.Gauge,
		Help: "Information about the pod",
		GenerateFunc: wrapPodFunc(func(p *corev1.Pod) *kbsm.Family {
			meterDefDomain := p.ObjectMeta.Annotations["meter_def_domain"]
			meterDefKind := p.ObjectMeta.Annotations["meter_def_kind"]
			meterDefVersion := p.ObjectMeta.Annotations["meter_def_version"]

			hostIP := p.Status.HostIP
			podIP := p.Status.PodIP
			podUID := string(p.UID)
			node := p.Spec.NodeName
			priorityClass := p.Spec.PriorityClassName
			hostNetwork := strconv.FormatBool(p.Spec.HostNetwork)

			m := kbsm.Metric{
				LabelKeys:   []string{"host_ip", "pod_ip", "pod_uid", "node", "meter_def_kind", "meter_def_version", "meter_def_domain", "priority_class", "host_network"},
				LabelValues: []string{hostIP, podIP, podUID, node, meterDefKind, meterDefVersion, meterDefDomain, priorityClass, hostNetwork},
				Value:       1,
			}

			return &kbsm.Family{
				Metrics: []*kbsm.Metric{&m},
			}
		}),
	},
}

// wrapPodFunc is a helper function for generating pod-based metrics
func wrapPodFunc(f func(*v1.Pod) *kbsm.Family) func(interface{}) *kbsm.Family {
	return func(obj interface{}) *kbsm.Family {
		pod := obj.(*v1.Pod)

		metricFamily := f(pod)

		for _, m := range metricFamily.Metrics {
			m.LabelKeys = append(descPodLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{pod.Namespace, pod.Name}, m.LabelValues...)
		}

		return metricFamily
	}
}
