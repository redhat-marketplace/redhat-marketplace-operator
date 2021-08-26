/*
Copyright 2020 IBM Co..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package marketplace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	opsrcv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	osappsv1 "github.com/openshift/api/apps/v1"
	osimagev1 "github.com/openshift/api/image/v1"
	marketplaceredhatcomv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplaceredhatcomv1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	// "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/inject"loo
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var k8sManager ctrl.Manager
var k8sScheme *runtime.Scheme
var dcControllerMockServer *ghttp.Server

const (
	imageStreamID string = "rhm-meterdefinition-file-server:v1"
	imageStreamTag string = "v1"
)


func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))
	os.Setenv("KUBEBUILDER_CONTROLPLANE_START_TIMEOUT", "2m")
	os.Setenv("POD_NAMESPACE","default")
	os.Setenv("IMAGE_STREAM_ID",imageStreamID)
	os.Setenv("IMAGE_STREAM_TAG",imageStreamTag)

	dcControllerMockServer = ghttp.NewServer()
	dcControllerMockServer.SetAllowUnhandledRequests(true)

	dcControllerMockServerAddr := fmt.Sprintf("%s%s","http://",dcControllerMockServer.Addr())
	os.Setenv("CATALOG_URL", dcControllerMockServerAddr)

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:  []string{filepath.Join("..", "..", "config", "crd", "bases")},
		KubeAPIServerFlags: append(envtest.DefaultKubeAPIServerFlags, "--bind-address=127.0.0.1"),
	}

	var err error
	k8sScheme = provideScheme()
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = marketplaceredhatcomv1alpha1.AddToScheme(k8sScheme)
	Expect(err).NotTo(HaveOccurred())

	err = marketplaceredhatcomv1beta1.AddToScheme(k8sScheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme
	k8sClient, err = client.New(cfg, client.Options{Scheme: k8sScheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: k8sScheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&RemoteResourceS3Reconciler{
		Client: k8sManager.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("RemoteResourceS3"),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	operatorConfig, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	factory := manifests.NewFactory(operatorConfig,k8sScheme)
	
	catalogClient,err := catalog.ProvideCatalogClient(operatorConfig)
	Expect(err).NotTo(HaveOccurred())

	catalogClient.UseInsecureClient()

	restConfig := k8sManager.GetConfig()

	clientset, err := kubernetes.NewForConfig(restConfig)
	Expect(err).NotTo(HaveOccurred())
	dc := &osappsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.DEPLOYMENT_CONFIG_NAME,
			Namespace: "default",
		},
		Spec: osappsv1.DeploymentConfigSpec{
			Triggers: osappsv1.DeploymentTriggerPolicies{
				{
					Type: osappsv1.DeploymentTriggerOnConfigChange,	
					ImageChangeParams: &osappsv1.DeploymentTriggerImageChangeParams{
						Automatic: true,
						ContainerNames: []string{"rhm-meterdefinition-file-server"},
						From: corev1.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "rhm-meterdefinition-file-server:v1",
						},
					},
				},
			},
		},
		Status: osappsv1.DeploymentConfigStatus{
			LatestVersion: 1,
			Conditions: []osappsv1.DeploymentCondition{
				{
					Type: osappsv1.DeploymentConditionType(osappsv1.DeploymentAvailable),
					Reason: "NewReplicationControllerAvailable",
					Status: corev1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					LastUpdateTime: metav1.Now(),
				},
			},
		},
	}
	
	is := &osimagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.DEPLOYMENT_CONFIG_NAME,
			Namespace: "default",
		},
		Spec:  osimagev1.ImageStreamSpec{
			LookupPolicy: osimagev1.ImageLookupPolicy{
				Local: false,
			},
			Tags: []osimagev1.TagReference{
				{
					Annotations: map[string]string{
						"openshift.io/imported-from": "quay.io/mxpaspa/rhm-meterdefinition-file-server:return-204-1.0.0",
					},
					From: &corev1.ObjectReference{
						Name: "quay.io/mxpaspa/rhm-meterdefinition-file-server:return-204-1.0.0",
						Kind: "DockerImage",
					},
					ImportPolicy: osimagev1.TagImportPolicy{
						Insecure: true,
						Scheduled: true,
					},
					Name: "v1",
					ReferencePolicy: osimagev1.TagReferencePolicy{
						Type: osimagev1.SourceTagReferencePolicy,
					},
					Generation: ptr.Int64(1),
				},
			},
		},
	}
	
	Expect(k8sClient.Create(context.TODO(), dc)).Should(Succeed(),"create test deploymentconfig")
	Expect(k8sClient.Create(context.TODO(), is)).Should(Succeed(),"create test image stream")

	err = (&DeploymentConfigReconciler{
		Client: k8sManager.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("DeploymentConfigReconciler"),
		Scheme: k8sManager.GetScheme(),
		cfg: operatorConfig,
		factory: factory,
		CatalogClient: catalogClient,
		KubeInterface: clientset,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		fmt.Println(err)
		Expect(err).ToNot(HaveOccurred())
	}()
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	dcControllerMockServer.Close()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func provideScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(openshiftconfigv1.AddToScheme(scheme))
	utilruntime.Must(olmv1.AddToScheme(scheme))
	utilruntime.Must(opsrcv1.AddToScheme(scheme))
	utilruntime.Must(olmv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(marketplaceredhatcomv1alpha1.AddToScheme(scheme))
	utilruntime.Must(marketplaceredhatcomv1beta1.AddToScheme(scheme))
	utilruntime.Must(osappsv1.AddToScheme(scheme))
	utilruntime.Must(osimagev1.AddToScheme(scheme))
	return scheme
}


