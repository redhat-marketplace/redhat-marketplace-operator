package client_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	rhmClient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"k8s.io/apimachinery/pkg/api/meta"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var _ = Describe("FindOwner", func() {

	var (
		ownerHelper *rhmClient.FindOwnerHelper
		// name, namespace string
		inClient   dynamic.Interface
		restMapper meta.RESTMapper
		restConfig *rest.Config
		// lookupOwner, owner *metav1.OwnerReference
	)

	BeforeEach(func() {
		restConfig, _ = config.GetConfig()
		inClient, _ = dynamic.NewForConfig(restConfig)
		restMapper, _ = managers.NewDynamicRESTMapper(restConfig)
		ownerHelper = rhmClient.NewFindOwnerHelper(inClient, restMapper)
	})

	Describe("NewFindOwnerHelper function test", func() {
		Context("Get Client", func() {
			It("Should be the same as original dynamic.Interface", func() {
				Expect(ownerHelper.GetClient()).To(Equal(inClient))
			})
		})
		Context("Get restMapper", func() {
			It("Should be nil", func() {
				Expect(ownerHelper.GetRestMapper()).To(BeNil())
				// Is this always expected to be nil?
			})
		})
	})

	// Describe("FindOnwer function test", func() {
	// 	BeforeEach(func() {
	// 		namespace = "redhat-marketplace-operator"
	// 		name = "rhm-marketplace"
	// 	})
	// 	Context("When the function succeeeds, and the OwnerReference is returned", func(){
	// 		It("Should return the OwnerReference", func(){

	// 		})
	// 	})
	// 	Context("When the function fails, and an error is returned", func(){
	// 		It("Should fail to get mapping", func(){
	// 			Expect(err).To(HaveOccurred())
	// 		})
	// 		It("Should fail to get resources", func(){
	// 			Expect(err).To(HaveOccurred())
	// 		})
	// 		It("Should fail to get meta.Accessor", func(){
	// 			Expect(err).To(HaveOccurred())
	// 		})
	// 	})
	// })
})
