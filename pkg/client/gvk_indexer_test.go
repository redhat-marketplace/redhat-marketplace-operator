package client_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	// marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	rhmClient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	// corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("FindOwner", func() {

	var (
		fieldIndexer client.FieldIndexer
		err          error
		objs         []runtime.Object
		namespace    string
	)

	BeforeEach(func() {
		namespace = "testingNamespace"
	})

	Describe("AddOwningControllerIndex function test", func() {
		var (
			marketplaceconfig = utils.BuildMarketplaceConfigCR(namespace, "example-id")
			razeedeployment   = utils.BuildRazeeCr(namespace, marketplaceconfig.Spec.ClusterUUID, marketplaceconfig.Spec.DeploySecretName)
			meterbase         = utils.BuildMeterBaseCr(namespace)
		)

		BeforeEach(func() {
			objs = []runtime.Object{
				marketplaceconfig,
				razeedeployment,
				meterbase,
			}
		})

		Context("Should successfully add Owning Controller Index", func() {
			It("Error should be nil", func() {
				err = rhmClient.AddOwningControllerIndex(fieldIndexer, objs)
				Expect(err).To(BeNil())
			})
		})
		Context("Should fail to add Owning Controller Index", func() {
			It("Error should not be nil", func() {
				err = rhmClient.AddOwningControllerIndex(fieldIndexer, objs)
				Expect(err).To(BeNil())
			})
		})
	})

})
