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

package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"

	// +kubebuilder:scaffold:imports
	osconfigv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Catalog Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var k8scache cache.Cache
var catalogClient *CatalogClient

const listenerAddress string = "127.0.0.1:2010"

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	catalogClientMockServerAddr := fmt.Sprintf("%s%s", "http://", listenerAddress)
	os.Setenv("CATALOG_URL", catalogClientMockServerAddr)

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "..", "tests", "v2", "testdata"),
		},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := runtime.NewScheme()
	err = v1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = v1beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = osconfigv1.Install(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})

	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mapper, err := apiutil.NewDiscoveryRESTMapper(cfg)
	Expect(err).NotTo(HaveOccurred())
	k8scache, err = cache.New(cfg,
		cache.Options{
			Scheme:    scheme,
			Mapper:    mapper,
			Resync:    nil,
			Namespace: "",
		})
	Expect(err).NotTo(HaveOccurred())

	k8sScheme := provideScheme()

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: k8sScheme,
	})
	Expect(err).ToNot(HaveOccurred())
	restConfig := k8sManager.GetConfig()

	clientset, err := kubernetes.NewForConfig(restConfig)
	Expect(err).NotTo(HaveOccurred())

	operatorConfig, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	catalogClient, err = ProvideCatalogClient(k8sClient, operatorConfig, clientset, ctrl.Log)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		k8scache.Start(ctx.Done())
	}()
}, 60)

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func provideScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	// utilruntime.Must(openshiftconfigv1.AddToScheme(scheme))
	// utilruntime.Must(olmv1.AddToScheme(scheme))
	// utilruntime.Must(opsrcv1.AddToScheme(scheme))
	// utilruntime.Must(olmv1alpha1.AddToScheme(scheme))
	// utilruntime.Must(monitoringv1.AddToScheme(scheme))
	// utilruntime.Must(marketplaceredhatcomv1alpha1.AddToScheme(scheme))
	// utilruntime.Must(marketplaceredhatcomv1beta1.AddToScheme(scheme))
	// utilruntime.Must(osappsv1.AddToScheme(scheme))
	// utilruntime.Must(osimagev1.AddToScheme(scheme))
	return scheme
}
