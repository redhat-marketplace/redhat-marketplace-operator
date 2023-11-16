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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	osappsv1 "github.com/openshift/api/apps/v1"
	marketplaceredhatcomv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplaceredhatcomv1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/rhmotransport"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var operatorCfg *config.OperatorConfig
var k8sClient client.Client
var testEnv *envtest.Environment
var k8sManager ctrl.Manager
var k8sScheme *runtime.Scheme
var factory *manifests.Factory
var clientset *kubernetes.Clientset
var authBuilderConfig *rhmotransport.AuthBuilderConfig
var catalogClient *catalog.CatalogClient
var doneChan chan struct{}

const (
	imageStreamID     string = "rhm-meterdefinition-file-server:v1"
	imageStreamTag    string = "v1"
	listenerAddress   string = "127.0.0.1:2100"
	operatorNamespace string = "redhat-marketplace"

	timeout  = time.Second * 50
	interval = time.Second * 5
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	os.Setenv("KUBEBUILDER_CONTROLPLANE_START_TIMEOUT", "2m")
	os.Setenv("POD_NAMESPACE", operatorNamespace)
	os.Setenv("IMAGE_STREAM_ID", imageStreamID)
	os.Setenv("IMAGE_STREAM_TAG", imageStreamTag)

	dcControllerMockServerAddr := fmt.Sprintf("%s%s", "http://", listenerAddress)
	os.Setenv("CATALOG_URL", dcControllerMockServerAddr)

	doneChan = make(chan struct{})
	By("bootstrapping test environment " + os.Getenv("KUBEBUILDER_ASSETS"))

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "..", "tests", "v2", "testdata"),
		},
	}

	k8sScheme = provideScheme()

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	operatorCfg, err = config.GetConfig()
	Expect(err).To(Succeed())
	operatorCfg.ReportController.PollTime = 5 * time.Second

	operatorCfg.DeployedNamespace = operatorNamespace

	// +kubebuilder:scaffold:scheme
	k8sClient, err = client.New(cfg, client.Options{Scheme: k8sScheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: k8sScheme,
	})
	Expect(err).ToNot(HaveOccurred())

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorNamespace,
		},
	}

	Expect(k8sClient.Create(context.TODO(), ns)).Should(Succeed(), "create operator namespace")

	// create required resources
	omns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.OPERATOR_MKTPLACE_NS,
		},
	}

	Expect(k8sClient.Create(context.TODO(), omns)).Should(Succeed(), "create openshift-marketplace namespace")

	// Deployment is necessary for marketplaceconfig or clusterregistration controller to set marketplaceconfig controller reference
	ls := map[string]string{"redhat.marketplace.com/name": "redhat-marketplace-operator"}
	replicas := int32(0)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RHM_METERING_DEPLOYMENT_NAME,
			Namespace: operatorNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: "manager",
						Name:  "manager",
					}},
				},
			},
		},
	}

	Expect(k8sClient.Create(context.TODO(), dep)).Should(Succeed(), "create controller deployment")

	factory = manifests.NewFactory(operatorCfg, k8sScheme)

	clientset, err = kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	authBuilderConfig = rhmotransport.ProvideAuthBuilder(k8sClient, operatorCfg, clientset, ctrl.Log)

	catalogClient, err = catalog.ProvideCatalogClient(authBuilderConfig, operatorCfg, ctrl.Log)
	Expect(err).ToNot(HaveOccurred())

	catalogClient.UseInsecureClient()

	err = (&DeploymentConfigReconciler{
		Client:        k8sClient,
		Log:           ctrl.Log.WithName("controllers").WithName("DeploymentConfigReconciler"),
		Scheme:        k8sScheme,
		Cfg:           operatorCfg,
		Factory:       factory,
		CatalogClient: catalogClient,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&MarketplaceConfigReconciler{
		Client: k8sClient,
		Log:    ctrl.Log.WithName("controllers").WithName("MarketplaceConfigReconciler"),
		Scheme: k8sScheme,
		Cfg:    operatorCfg,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ClusterServiceVersionReconciler{
		Client: k8sClient,
		Log:    ctrl.Log.WithName("controllers").WithName("ClusterServiceVersionReconciler"),
		Scheme: k8sScheme,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&MeterDefinitionReconciler{
		Client: k8sClient,
		Log:    ctrl.Log.WithName("controllers").WithName("MeterDefinitionReconciler"),
		Scheme: k8sScheme,
		Cfg:    operatorCfg,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		gexec.KillAndWait(4 * time.Second)

		// Teardown the test environment once controller is fnished.
		// Otherwise from Kubernetes 1.21+, teardon timeouts waiting on
		// kube-apiserver to return
		err := testEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	/*
			From Kubernetes 1.21+, when it tries to cleanup the test environment, there is
			a clash if a custom controller is created during testing. It would seem that
			the controller is still running and kube-apiserver will not respond to shutdown.
			This is the reason why teardown happens in BeforeSuite() after controller has stopped.
			The error shown is as documented in:
			https://github.com/kubernetes-sigs/controller-runtime/issues/1571
		/*
		/*
			By("tearing down the test environment")
			err := testEnv.Stop()
			Expect(err).NotTo(HaveOccurred())
	*/

	/*
		close(doneChan)
		By("tearing down the test environment")
		err := testEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
	*/
})

func provideScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(openshiftconfigv1.AddToScheme(scheme))
	utilruntime.Must(olmv1.AddToScheme(scheme))
	utilruntime.Must(olmv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(marketplaceredhatcomv1alpha1.AddToScheme(scheme))
	utilruntime.Must(marketplaceredhatcomv1beta1.AddToScheme(scheme))
	utilruntime.Must(osappsv1.AddToScheme(scheme))
	mktypes.RegisterImageStream(scheme)
	return scheme
}
