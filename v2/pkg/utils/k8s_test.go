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

	"emperror.dev/errors"
	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo/v2"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Testing with Ginkgo", func() {

	It("persistent volume claim", func() {

		// setup returns a fakeClient for testing purposes

		// Setup resources we want to retrieve

		// Creating those resources

		pvc, err := NewPersistentVolumeClaim(PersistentVolume{})

		if err != nil {
			GinkgoT().Errorf("failed with error %v", err)
		}

		if len(pvc.Spec.AccessModes) == 0 {
			GinkgoT().Error("no defined access modes")
		}

		if pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
			GinkgoT().Errorf("expect %v but got %v", corev1.ReadWriteOnce, pvc.Spec.AccessModes[0])
		}

		val := corev1.ReadWriteMany
		pvc, err = NewPersistentVolumeClaim(PersistentVolume{
			AccessMode: &val,
		})

		if err != nil {
			GinkgoT().Errorf("failed with error %v", err)
		}

		if len(pvc.Spec.AccessModes) == 0 {
			GinkgoT().Error("no defined access modes")
		}

		if pvc.Spec.AccessModes[0] != corev1.ReadWriteMany {
			GinkgoT().Errorf("expect %v but got %v", corev1.ReadWriteMany, pvc.Spec.AccessModes[0])
		}
	})
	It("filter by namespace", func() {

		rclient := setup()
		setupResources(rclient)
		var err error

		// Retrieve and compare cases
		ns := []corev1.Namespace{}
		// case 1:
		// get resources in case an empty list of namespaces is passed
		// should return: runtime.object list of size 1
		podList1 := &corev1.PodList{}
		err = FilterByNamespace(podList1, ns, rclient)
		if err != nil {
			GinkgoT().Error(err, "Could not execute FilterByNamespace")
		} else if len(podList1.Items) != 2 {
			GinkgoT().Error("Case 1: Did not return the correct number of resrouces. Expected: 2. Actual: ", len(podList1.Items))
		}

		// case 2:
		// get resources in case a list with a single namespace is passed
		// should return: runtime.object list of size 1
		ns = append(ns, *testNs1)
		serviceMonitorList1 := &monitoringv1.ServiceMonitorList{}
		err = FilterByNamespace(serviceMonitorList1, ns, rclient)
		if err != nil {
			GinkgoT().Error(err, "Could not execute FilterByNamespace")
		} else if len(serviceMonitorList1.Items) != 1 {
			GinkgoT().Error("Case 2: Did not return the correct number of resrouces. Expected: 1. Actual: ", len(serviceMonitorList1.Items))
		}

		// case 3:
		// get resources in case a list with multiple namespaces is passed
		// should return: runtime.object list of size 2
		ns = append(ns, *testNs2)
		podList2 := &corev1.PodList{}
		err = FilterByNamespace(podList2, ns, rclient)
		if err != nil {
			GinkgoT().Error(err, "Could not execute FilterByNamespace")
		} else if len(podList2.Items) != 2 {
			GinkgoT().Error("Case 3: Did not return the correct number of resrouces. Expected: 2. Actual: ", len(podList2.Items))
		}

		// case 4:
		// get resources in case a runtime.object with no existsing resources are available is passed
		// should return: an runtime.object list of 0 resourecs
		serviceList1 := &corev1.ServiceList{}
		err = FilterByNamespace(serviceList1, ns, rclient)
		if err != nil {
			GinkgoT().Error(err, "Could not execute FilterByNamespace")
		} else if len(serviceList1.Items) != 0 {
			GinkgoT().Error("Case 4: Did not return the correct number of resrouces. Expected: 0. Actual: ", len(serviceList1.Items))
		}

		// case 5:
		// passing a ServiceMonitorList with a set of ListOption with no namespace
		// should return: an runtime.object list of 1 resources
		ns2 := []corev1.Namespace{}
		ns2 = append(ns2, *testNs1)
		serviceMonitorList2 := &monitoringv1.ServiceMonitorList{}
		var listOpts1 []client.ListOption
		listOpts1 = append(listOpts1, client.InNamespace(testNs2.ObjectMeta.Name))
		// listOpts1 := client.InNamespace(testNs2.ObjectMeta.Name)
		err = FilterByNamespace(serviceMonitorList2, ns2, rclient, listOpts1...)
		if err != nil {
			GinkgoT().Error(err, "Could not execute FilterByNamespace")
		} else if len(serviceMonitorList2.Items) != 1 {
			GinkgoT().Error("Case 5: Did not return the correct number of resrouces. Expected: 1. Actual: ", len(serviceMonitorList2.Items))
		}

		// case 6:
		// get resources in case a list with a single namespace is passed
		// should return: runtime.object list of size n
		ns3 := []corev1.Namespace{*testNs1, *testNs2}
		serviceMonitorList3 := &monitoringv1.ServiceMonitorList{}
		err = FilterByNamespace(serviceMonitorList3, ns3, rclient)
		if err != nil {
			GinkgoT().Error(err, "Could not execute FilterByNamespace")
		} else if len(serviceMonitorList3.Items) != 2 {
			GinkgoT().Error("Case 2: Did not return the correct number of resrouces. Expected: 1. Actual: ", len(serviceMonitorList1.Items))
		}

		// case 7:
		// get resources for servicelist
		// should return: runtime.object list of size 1
		ns3 = append(ns3, *testNs3)
		serviceList := &corev1.ServiceList{}
		err = FilterByNamespace(serviceList, ns3, rclient)
		assert.NoError(GinkgoT(), err)
		assert.Len(GinkgoT(), serviceList.Items, 1)

		// case 8:
		// test no supported list
		// should return: error
		nodeList := &corev1.NodeList{}
		err = FilterByNamespace(nodeList, ns3, rclient)
		assert.EqualError(GinkgoT(), err, "type is not supported for filter aggregation")
	})
	It("get resources", func() {

		rclient := setup()
		err := setupResources(rclient)

		assert.NoError(GinkgoT(), err)

		podList3 := &corev1.PodList{}
		opts1 := []client.ListOption{
			client.InNamespace(""),
		}

		// If namespace was passed via ListOption
		err = getResources(podList3, opts1, rclient)
		if err != nil {
			GinkgoT().Error(err, "Could not execute GetResources")
		} else if len(podList3.Items) != 2 {
			GinkgoT().Error("Case 1: expected: 2. Actual: ", len(podList3.Items))
		}

		// If ListOption with conflicting namespaces is passed, only use latest one
		opts1 = append(opts1, client.InNamespace(testNamespace1))
		err = getResources(podList3, opts1, rclient)
		if err != nil {
			GinkgoT().Error(err, "Could not execute GetResources")
		} else if len(podList3.Items) != 1 {
			GinkgoT().Error("Case 2: expected: 1. Actual: ", len(podList3.Items))
		}

		// If ListOptions with conflicting namespaces is passed, only use latest one
		opts1 = append(opts1, client.InNamespace(""))
		err = getResources(podList3, opts1, rclient)
		if err != nil {
			GinkgoT().Error(err, "Could not execute GetResources")
		} else if len(podList3.Items) != 2 {
			GinkgoT().Error("Case 3: expected: 2. Actual: ", len(podList3.Items))
		}
	})
})
var (
	namespace               = "redhat-marketplace-operator"
	customerID       string = "example-userid"
	testNamespace1          = "testing-namespace-1"
	testNamespace2          = "testing-namespace-2"
	testNamespace3          = "testing-namespace-3"
	storageClassName        = "rook-ceph"
	features                = &common.Features{
		Deployment:                         ptr.Bool(true),
		EnableMeterDefinitionCatalogServer: ptr.Bool(true),
	}
	installIBMCatalogSource = ptr.Bool(true)

	marketplaceconfig = BuildMarketplaceConfigCR(testNamespace1, customerID)
	meterbase         *marketplacev1alpha1.MeterBase

	testNs1 = &corev1.Namespace{}
	testNs2 = &corev1.Namespace{}
	testNs3 = &corev1.Namespace{}
)

func setup() client.Client {
	marketplaceconfig.Spec.Features = features
	meterbase = BuildMeterBaseCr(testNamespace1, *marketplaceconfig.Spec.Features.EnableMeterDefinitionCatalogServer, &storageClassName)
	defaultFeatures := []string{"meterbase"}
	viper.Set("features", defaultFeatures)
	testNs1.ObjectMeta.Name = testNamespace1
	testNs2.ObjectMeta.Name = testNamespace2
	testNs3.ObjectMeta.Name = testNamespace3
	objs := []runtime.Object{
		marketplaceconfig,
		meterbase,
		testNs1,
		testNs2,
		testNs3,
	}
	s := scheme.Scheme
	_ = monitoringv1.AddToScheme(s)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, marketplaceconfig)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterbase)

	client := fake.NewFakeClient(objs...)
	return client
}

func setupResources(rclient client.Client) error {
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

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: testNamespace3,
		},
	}

	err := rclient.Create(context.TODO(), testPod1)
	if err != nil {
		return errors.Wrap(err, "create testPod1")
	}
	err = rclient.Create(context.TODO(), testPod2)
	if err != nil {
		return errors.Wrap(err, "create testPod2")
	}
	err = rclient.Create(context.TODO(), serviceMonitor1)
	if err != nil {
		return errors.Wrap(err, "create serviceMonitor1")
	}
	err = rclient.Create(context.TODO(), serviceMonitor2)
	if err != nil {
		return errors.Wrap(err, "create serviceMonitor2")
	}
	err = rclient.Create(context.TODO(), service)
	if err != nil {
		return errors.Wrap(err, "create service")
	}

	return nil
}
