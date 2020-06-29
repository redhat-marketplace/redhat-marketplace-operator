package controller_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/testenv"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestMeterbase(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Meterbase Suite")
}

var log = logf.Log.WithName("controller_suite")
var cfg *rest.Config
var k8sClient client.Client
var k8sManager manager.Manager
var testEnv *envtest.Environment
var namespace = "openshift-redhat-marketplace"

var _ = BeforeSuite(func(done Done) {
	testenv.SetupTestEnv(log, cfg, k8sClient, k8sManager, testEnv, namespace, done)
}, 60)

var _ = AfterSuite(func() {
	testenv.TeardownTestEnv(testEnv)
})
