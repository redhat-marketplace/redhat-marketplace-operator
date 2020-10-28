package testenv

import (
	"context"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kubeletMonitor = &monitoringv1.ServiceMonitor{
		ObjectMeta: v1.ObjectMeta{
			Name:      "kubelet",
			Namespace: "openshift-monitoring",
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: "web",
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		},
	}

	kubeStateMonitor = &monitoringv1.ServiceMonitor{
		ObjectMeta: v1.ObjectMeta{
			Name:      "kube-state-metrics",
			Namespace: "openshift-monitoring",
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: "web",
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		},
	}
)

func CreatePrereqs() {
	Expect(k8sClient.Create(context.TODO(), kubeletMonitor)).Should(SucceedOrAlreadyExist)
	Expect(k8sClient.Create(context.TODO(), kubeStateMonitor)).Should(SucceedOrAlreadyExist)
}

func DeletePrereqs() {
	k8sClient.Delete(context.TODO(), kubeletMonitor)
	k8sClient.Delete(context.TODO(), kubeStateMonitor)
}
