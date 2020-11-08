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

package testenv

import (
	"context"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
