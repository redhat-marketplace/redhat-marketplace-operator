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

package utils

import (
	"context"
	"testing"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	name                  = "markeplaceconfig"
	namespace             = "redhat-marketplace-operator"
	customerID     string = "example-userid"
	testNamespace1        = "testing-namespace-1"
	testNamespace2        = "testing-namespace-2"

	marketplaceconfig = buildMarketplaceConfigCR(name, namespace, customerID)
	razeedeployment   = BuildRazeeCr(namespace, marketplaceconfig.Spec.ClusterUUID, marketplaceconfig.Spec.DeploySecretName)
	meterbase         = BuildMeterBaseCr(namespace)
)

func TestPersistenVolumeClaim(t *testing.T) {
	pvc, err := NewPersistentVolumeClaim(PersistentVolume{})

	if err != nil {
		t.Errorf("failed with error %v", err)
	}

	if len(pvc.Spec.AccessModes) == 0 {
		t.Error("no defined access modes")
	}

	if pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("expect %v but got %v", corev1.ReadWriteOnce, pvc.Spec.AccessModes[0])
	}

	val := corev1.ReadWriteMany
	pvc, err = NewPersistentVolumeClaim(PersistentVolume{
		AccessMode: &val,
	})

	if err != nil {
		t.Errorf("failed with error %v", err)
	}

	if len(pvc.Spec.AccessModes) == 0 {
		t.Error("no defined access modes")
	}

	if pvc.Spec.AccessModes[0] != corev1.ReadWriteMany {
		t.Errorf("expect %v but got %v", corev1.ReadWriteMany, pvc.Spec.AccessModes[0])
	}
}

func TestFilterByNamespace(t *testing.T) {

	//Setup fake client
	defaultFeatures := []string{"razee", "meterbase"}
	viper.Set("assets", "../../../assets")
	viper.Set("features", defaultFeatures)
	objs := []runtime.Object{
		marketplaceconfig,
		razeedeployment,
		meterbase,
	}
	s := scheme.Scheme
	_ = monitoringv1.AddToScheme(s)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, marketplaceconfig)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, razeedeployment)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterbase)
	client := fake.NewFakeClient(objs...)

	// Setup resources we want to retrieve
	resourceList1 := corev1.ResourceList{}
	resourceList2 := corev1.ResourceList{}
	resourceList3 := corev1.ResourceList{}
	testNs1 := &corev1.Namespace{}
	testNs1.ObjectMeta.Name = testNamespace1
	testNs2 := &corev1.Namespace{}
	testNs2.ObjectMeta.Name = testNamespace2

	testPod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: testNamespace1,
		},
	}
	testPod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: testNamespace2,
		},
	}

	serviceMonitor1 := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-servicemonitor-1",
			Namespace: testNamespace1,
		},
	}

	serviceMonitor2 := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-servicemonitor-2",
			Namespace: testNamespace2,
		},
	}

	// Creating those resources
	err := client.Create(context.TODO(), testNs1)
	if err != nil {
		t.Error("could not setup test, error creating testing-namespace")
	}
	err = client.Create(context.TODO(), testNs2)
	if err != nil {
		t.Error("could not setup test, error creating testing-namespace")
	}
	err = client.Create(context.TODO(), testPod1)
	if err != nil {
		t.Error("could not setup test, error creating testing-pod")
	}
	err = client.Create(context.TODO(), testPod2)
	if err != nil {
		t.Error("could not setup test, error creating testing-pod")
	}
	err = client.Create(context.TODO(), serviceMonitor1)
	if err != nil {
		t.Error("could not setup test, error creating serviceMonitor")
	}
	err = client.Create(context.TODO(), serviceMonitor2)
	if err != nil {
		t.Error("could not setup test, error creating serviceMonitor")
	}

	// Retrieve and compare cases
	ns := []corev1.Namespace{}
	// case 1:
	// get resources in case an empty list of namespaces is passed
	// should return: at least a resourceList with at least 4 resources
	err, resourceList1 = FilterByNamespace(ns, resourceList1, client)
	if err != nil {
		t.Error(err, "Could not execute FilterByNamespace")
	} else if len(resourceList1) < 4 {
		t.Error("Did not return the correct number of resrouces. Expected: minimum of 4. Actual: ", len(resourceList1))
	}

	// case 2:
	// get resources in case a list with a single namespace is passed
	// should return: at resourceList of 2 resource
	ns = append(ns, *testNs1)
	err, resourceList2 = FilterByNamespace(ns, resourceList2, client)
	if err != nil {
		t.Error(err, "Could not execute FilterByNamespace")
	} else if len(resourceList2) != 2 {
		t.Error("Did not return the correct number of resrouces. Expected: 2. Actual: ", len(resourceList2))
	}

	// case 3:
	// get resources in case a list with multiple namespaces is passed
	// should return: resourceList of 4 resources
	ns = append(ns, *testNs2)
	err, resourceList3 = FilterByNamespace(ns, resourceList3, client)
	if err != nil {
		t.Error(err, "Could not execute FilterByNamespace")
	} else if len(resourceList3) != 4 {
		t.Error("Did not return the correct number of resrouces. Expected: 4. Actual: ", len(resourceList3))
	}

}

func buildMarketplaceConfigCR(name, namespace, customerID string) *marketplacev1alpha1.MarketplaceConfig {
	return &marketplacev1alpha1.MarketplaceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MarketplaceConfigSpec{
			RhmAccountID: customerID,
		},
	}
}
