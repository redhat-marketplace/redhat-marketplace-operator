package apis_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/harness"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/testenv"
)

var testHarness *harness.TestHarness

func TestApis(t *testing.T) {
	RegisterFailHandler(harness.PodFailHandler(testHarness))
	RunSpecs(t, "Apis Suite")
}

var _ = BeforeSuite(func() {
	var err error
	By("setting up env")
	testHarness, err = harness.NewTestHarness(harness.TestHarnessOptions{
		EnabledFeatures: []string{},
		Namespace:       "openshift-redhat-marketplace",
		WatchNamespace:  "",
		ProvideScheme:   testenv.InitializeScheme,
	})
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%v+", err))

	_, err = testHarness.Start()
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	testHarness.Stop()
}, 60)
