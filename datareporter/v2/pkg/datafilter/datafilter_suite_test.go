// Copyright 2024 IBM Corp.
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

package datafilter_test

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/datafilter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	testEnv     *envtest.Environment
	cfg         *rest.Config
	log         logr.Logger
	k8sClient   client.Client
	k8sManager  ctrl.Manager
	dataFilters *datafilter.DataFilters
)

var _ = BeforeSuite(func() {
	var err error

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
	}

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	rc := retryablehttp.NewClient()
	sc := rc.StandardClient()
	dataFilters = datafilter.NewDataFilters(ctrl.Log.WithName("datafilter"), k8sClient, sc)

	// k8s Configuration Objects

	destHeaderMap := make(map[string]string)
	destHeaderMap["accept"] = "*/*"
	destHeaderSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dest-header-map-secret",
			Namespace: "default",
		},
		StringData: destHeaderMap,
	}
	err = k8sClient.Create(context.TODO(), &destHeaderSecret)
	Expect(err).ToNot(HaveOccurred())

	authHeaderMap := make(map[string]string)
	authHeaderMap["accept"] = "application/json"
	authHeaderMap["Content-Type"] = "application/json"
	authHeaderSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-header-map-secret",
			Namespace: "default",
		},
		StringData: authHeaderMap,
	}
	err = k8sClient.Create(context.TODO(), &authHeaderSecret)
	Expect(err).ToNot(HaveOccurred())

	authDataMap := make(map[string]string)
	authDataMap["auth"] = `{"token": "eyJraWQiOiIx..."}`
	authDataSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-data-secret",
			Namespace: "default",
		},
		StringData: authDataMap,
	}
	err = k8sClient.Create(context.TODO(), &authDataSecret)
	Expect(err).ToNot(HaveOccurred())

	kazaamMap := make(map[string]string)
	kazaamMap["kazaam.json"] = `[{"operation": "shift", "spec": {"instances[0].endTime": "timestamp", "instances[0].instanceId": "properties.source", "instances[0].metricUsage[0].metricId": "properties.unit", "instances[0].metricUsage[0].quantity": "properties.quantity", "instances[0].startTime": "timestamp", "subscriptionId": "properties.productId"}}]`
	kazaamConfigMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kazaam-configmap",
			Namespace: "default",
		},
		Data: kazaamMap,
	}
	err = k8sClient.Create(context.TODO(), &kazaamConfigMap)
	Expect(err).ToNot(HaveOccurred())

	kazaamMapBad := make(map[string]string)
	kazaamMapBad["kazaam.json"] = `[{"operation": "INVALID", "spec": {"instances[0].endTime": "timestamp", "instances[0].instanceId": "properties.source", "instances[0].metricUsage[0].metricId": "properties.unit", "instances[0].metricUsage[0].quantity": "properties.quantity", "instances[0].startTime": "timestamp", "subscriptionId": "properties.productId"}}]`
	kazaamConfigMapBad := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kazaam-configmap-bad",
			Namespace: "default",
		},
		Data: kazaamMapBad,
	}
	err = k8sClient.Create(context.TODO(), &kazaamConfigMapBad)
	Expect(err).ToNot(HaveOccurred())

})

var _ = AfterSuite(func() {
	var err error
	err = testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

//stop testEnv

func TestDatafilter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Datafilter Suite")
}
