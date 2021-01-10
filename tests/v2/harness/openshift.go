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

package harness

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"emperror.dev/errors"
	"github.com/caarlos0/env/v6"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const FeatureMockOpenShift string = "MockOpenShift"

type mockOpenShift struct {
	Namespace string `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`

	cleanup []runtime.Object
}

func (e *mockOpenShift) Name() string {
	return FeatureMockOpenShift
}

func (e *mockOpenShift) Parse() error {
	return env.Parse(e)
}

func (e *mockOpenShift) HasCleanup() []runtime.Object {
	return e.cleanup
}

func (e *mockOpenShift) Setup(h *TestHarness) error {
	additionalNamespaces := []string{
		"openshift-monitoring",
		"openshift-config-managed",
		"openshift-config",
	}

	for _, namespace := range additionalNamespaces {
		ok, err := SucceedOrAlreadyExist.Match(
			h.Upsert(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}))

		if !ok {
			return errors.Wrapf(err, "failed to create namespace: %s", namespace)
		}
	}

	var (
		kubeletMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
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
			ObjectMeta: metav1.ObjectMeta{
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

	e.cleanup = []runtime.Object{kubeletMonitor, kubeStateMonitor}

	h.Upsert(context.TODO(), kubeletMonitor)
	h.Upsert(context.TODO(), kubeStateMonitor)

	var (
		tlsResources = []types.NamespacedName{
			{Namespace: h.Config.Namespace, Name: "prometheus-operator-tls"},
			{Namespace: h.Config.Namespace, Name: "rhm-metric-state-tls"},
			{Namespace: h.Config.Namespace, Name: "rhm-prometheus-meterbase-tls"},
		}
		certsResources = []types.NamespacedName{
			{Namespace: "openshift-config-managed", Name: "kubelet-serving-ca"},
			{Namespace: h.Config.Namespace, Name: "serving-certs-ca-bundle"},
			{Namespace: h.Config.Namespace, Name: "operator-certs-ca-bundle"},
		}
	)

	rootDir, _ := GetRootDirectory()

	serviceCA, err := ioutil.ReadFile(filepath.Join(rootDir, "test/certs/ca.crt"))
	if err != nil {
		return err
	}

	for _, resource := range certsResources {
		configmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: resource.Namespace,
				Name:      resource.Name,
				Annotations: map[string]string{
					"service.beta.openshift.io/inject-cabundle": "true",
				},
			},
			Data: map[string]string{
				"service-ca.crt": string(serviceCA),
			},
		}

		err := h.Upsert(context.TODO(), configmap)
		e.cleanup = append(e.cleanup, configmap)

		if err != nil {
			return err
		}
	}

	serverCrt, err := ioutil.ReadFile(filepath.Join(rootDir, "test/certs/server.pem"))
	if err != nil {
		return err
	}
	serverKey, err := ioutil.ReadFile(filepath.Join(rootDir, "test/certs/server-key.pem"))
	if err != nil {
		return err
	}

	for _, resource := range tlsResources {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: resource.Namespace,
				Name:      resource.Name,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"tls.crt": serverCrt,
				"tls.key": serverKey,
			},
		}

		err := h.Upsert(context.TODO(), secret)
		if err != nil {
			return err
		}
		e.cleanup = append(e.cleanup, secret)
	}

	return nil
}
