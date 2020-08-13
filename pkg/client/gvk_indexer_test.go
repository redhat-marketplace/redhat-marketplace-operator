package client_test

// import (
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"

// 	rhmClient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// 	// "k8s.io/apimachinery/pkg/runtime"
// 	corev1 "k8s.io/api/core/v1"
// 	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// )

// var _ = Describe("FindOwner", func() {

// 	var (
// 		fieldIndexer client.FieldIndexer
// 		err          error
// 		podList &corev1.PodList{}
// 	)

// 	Describe("AddOwningControllerIndex function test", func() {
// 		var(
// 			pass []
// 		)

// 		// BeforeEach(func(){
// 		// 	fieldIndexer
// 		// })

// 		Context("Should successfully add Owning Controller Index", func() {
// 			It("Error should be nil", func() {
// 				err = rhmClient.AddOwningControllerIndex(fieldIndexer, []corev1.Pod{})
// 				Expect(err).To(BeNil())
// 			})
// 		})
// 		Context("Should fail to add Owning Controller Index", func() {
// 			It("Error should not be nil", func() {
// 				err = rhmClient.AddOwningControllerIndex(fieldIndexer, &corev1.Pod{})
// 				Expect(err).To(BeNil())
// 			})
// 		})
// 	})

// })
