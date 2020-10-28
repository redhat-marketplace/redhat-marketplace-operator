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
	"io/ioutil"
	"sync"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

var (
	tlsResources = []types.NamespacedName{
		{Namespace: Namespace, Name: "prometheus-operator-tls"},
		{Namespace: Namespace, Name: "rhm-metric-state-tls"},
		{Namespace: Namespace, Name: "rhm-prometheus-meterbase-tls"},
	}
	certsResources = []types.NamespacedName{
		{Namespace: "openshift-config-managed", Name: "kubelet-serving-ca"},
		{Namespace: Namespace, Name: "serving-certs-ca-bundle"},
		{Namespace: Namespace, Name: "operator-certs-ca-bundle"},
	}
	createdObjects = []runtime.Object{}
	mutex          = sync.Mutex{}
)

func addCerts() {
	mutex.Lock()
	defer mutex.Unlock()

	serviceCA, err := ioutil.ReadFile("../certs/ca.pem")
	Expect(err).To(Succeed())

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

		Expect(k8sClient.Create(context.TODO(), configmap)).Should(SucceedOrAlreadyExist, "failed to create", "name", resource.Name)
		createdObjects = append(createdObjects, configmap)
	}

	serverCrt, err := ioutil.ReadFile("../certs/server.pem")
	Expect(err).To(Succeed())
	serverKey, err := ioutil.ReadFile("../certs/server-key.pem")
	Expect(err).To(Succeed())

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

		Expect(k8sClient.Create(context.TODO(), secret)).Should(SucceedOrAlreadyExist, "failed to create", "name", resource.Name)
		createdObjects = append(createdObjects, secret)
	}
}

func cleanupCerts() {
	mutex.Lock()
	defer mutex.Unlock()

	for _, created := range createdObjects {
		k8sClient.Delete(context.TODO(), created)
	}
}
