package testenv

import (
	"context"
	"go/build"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"

	//olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	//"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestTestenv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testenv Suite")
}

const namespace = "openshift-redhat-marketplace"

var cfg *rest.Config
var k8sClient client.Client
var k8sManager ctrl.Manager
var testEnv *envtest.Environment
var cc reconcileutils.ClientCommandRunner
var stop chan struct{}

func TestEnv(t *testing.T) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller EnvTest Suite",
		[]Reporter{printer.NewlineReporter{}})
}

func getModuleDirectory(pkgpath string) (*build.Package, error) {
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	dir = filepath.Join(dir, "..", "..")
	ctxt := build.Default
	ctxt.Dir = dir
	return ctxt.Import(
		pkgpath,
		dir,
		build.FindOnly,
	)

}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	Expect(os.Setenv("TEST_ASSET_KUBE_APISERVER", "../../testbin/kube-apiserver")).To(Succeed())
	Expect(os.Setenv("TEST_ASSET_ETCD", "../../testbin/etcd")).To(Succeed())
	Expect(os.Setenv("TEST_ASSET_KUBECTL", "../../testbin/kubectl")).To(Succeed())
	Expect(os.Setenv("WATCH_NAMESPACE", "")).To(Succeed())

	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	pkg, err := getModuleDirectory(reflect.TypeOf(olmv1.OperatorGroup{}).PkgPath())
	Expect(err).To(Succeed())

	olmPath := filepath.Join(pkg.PkgRoot, "..", "crds")
	Expect(olmPath).To(BeADirectory(), "olmPath")

	pkg, err = getModuleDirectory(reflect.TypeOf(monitoringv1.Prometheus{}).PkgPath())
	Expect(err).To(Succeed())
	monitoringPath := filepath.Join(
		pkg.PkgRoot,
		"..", "example", "prometheus-operator-crd",
	)
	Expect(monitoringPath).To(BeADirectory(), "monitoringPath")

	pkg, err = getModuleDirectory(reflect.TypeOf(opsrcv1.OperatorSource{}).PkgPath())
	Expect(err).To(Succeed())
	opsrcPath := filepath.Join(
		pkg.PkgRoot, "..", "manifests",
	)
	Expect(opsrcPath).To(BeADirectory())

	By("bootstrapping test environment")
	t := true
	if os.Getenv("TEST_USE_EXISTING_CLUSTER") == "true" {
		testEnv = &envtest.Environment{
			UseExistingCluster: &t,
		}
	} else {
		testEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "..", "deploy", "crds"),
				olmPath,
				monitoringPath,
			},
		}
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	ctrlMain, err := initializeMainCtrl(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(ctrlMain).ToNot(BeNil())

	stop = make(chan struct{})

	go func() {
		ctrlMain.Run(stop)
	}()

	k8sManager = ctrlMain.Manager
	k8sClient = ctrlMain.Manager.GetClient()
	Expect(k8sClient).ToNot(BeNil())
	Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})).To(Succeed())
	Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "openshift-monitoring",
		},
	})).To(Succeed())

	cc = reconcileutils.NewLoglessClientCommand(k8sClient, k8sManager.GetScheme())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("shutting down controller")
	close(stop)

	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)

	if testEnv != nil {
		err := testEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
	}
})

