package client_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	rhmClient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	// "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	// cmd "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("FindOwner", func() {

	var (
		ownerHelper     *rhmClient.FindOwnerHelper
		name, namespace string
		inClient        dynamic.Interface
		restMapper      meta.RESTMapper
		restConfig      *rest.Config
		testNs          *corev1.Namespace
		fakeClient      client.Client
		// lookupOwner, owner *metav1.OwnerReference

	)

	BeforeEach(func() {

		namespace = "test-ns"
		testNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		fakeClient = setup(testNs)
		// restConfig := cmd.NewConfig()
		restConfig, _ = config.GetConfig()
		inClient, _ = dynamic.NewForConfig(restConfig)
		restMapper, _ = managers.NewDynamicRESTMapper(restConfig)
		ownerHelper = rhmClient.NewFindOwnerHelper(inClient, restMapper)
	})

	Describe("NewFindOwnerHelper function test", func() {
		Context("Get RestConfig", func() {
			It("Should not be nil", func() {
				Expect(restConfig).NotTo(BeNil())
			})
		})
		Context("Get restMapper", func() {
			It("Should be the same as original restMapper", func() {
				Expect(ownerHelper.GetRestMapper()).To(Equal(restMapper))
			})
		})
		Context("Get Client", func() {
			It("Should be the same as original dynamic.Interface", func() {
				Expect(ownerHelper.GetClient()).To(Equal(inClient))
			})
		})
	})

	Describe("FindOwner function test", func() {
		var (
			err              error
			meterdef         *marketplacev1alpha1.MeterDefinition
			owner            *metav1.OwnerReference
			kind, apiVersion string
			pod              *corev1.Pod
		)
		BeforeEach(func() {

			namespace = "nam-test-1"
			name = "test-meterdef"
			wl := marketplacev1alpha1.Workload{
				Name: "podcpu",
			}
			meterdef = &marketplacev1alpha1.MeterDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: marketplacev1alpha1.MeterDefinitionSpec{
					Group:              "cockroach.db.com",
					Kind:               "Cockroachdb",
					WorkloadVertexType: "OperatorGroup",
					Workloads: []marketplacev1alpha1.Workload{
						wl,
					},
				},
			}
			apiVersion = "marketplace.redhat.com/v1alpha1"
			meterdef.APIVersion = apiVersion
			kind = "MeterDefinition"
			meterdef.Kind = kind
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: namespace,
				},
			}
			fakeClient.Create(context.TODO(), meterdef)
			owner = metav1.NewControllerRef(pod, meterdef.GroupVersionKind())

		})
		Context("When the function succeeeds, and the OwnerReference is returned", func() {
			It("Should return the OwnerReference", func() {
				owner, err = ownerHelper.FindOwner(name, namespace, owner)
				Expect(meterdef.APIVersion).To(Equal(apiVersion))
				Expect(err).To(BeNil())
			})
			JustAfterEach(func() {
				fmt.Println("Owner:", owner)
				fmt.Println("MeterDefinition:" + meterdef.GroupVersionKind().String())
				fmt.Println("G:" + meterdef.GroupVersionKind().Group)
				fmt.Println("V:" + meterdef.GroupVersionKind().Version)
				fmt.Println("K:" + meterdef.GroupVersionKind().Kind)

			})
		})
		Context("When the function fails, and an error is returned", func() {
			It("Should panic: OwnerReference is nil", func() {
				obj := interface{}(meterdef)
				meta, _ := obj.(metav1.Object)
				owner = metav1.GetControllerOf(meta)
				defer func() {
					if r := recover(); r == nil {
						Fail("Did not panic")
					}
				}()
				_, err = ownerHelper.FindOwner(name, namespace, owner)

			})
			// It("Should panic: Object.Kind is empty", func() {
			// 	defer func() {
			// 		if r := recover(); r == nil {
			// 			Fail("Did not panic")
			// 		}
			// 	}()
			// 	_, err = ownerHelper.FindOwner(name, namespace, owner)
			// })  When we're not logged in via oc
			It("Should fail to get mapping", func() {
				meterdef.APIVersion = "wrong/fake1"
				owner, err = ownerHelper.FindOwner(name, namespace, owner)
				// Expect(err).To(ContainSubstring("failed to get mapping", err)) FIX
				Expect(err).ToNot(BeNil())
			})
			PIt("Should fail to get resources", func() {
				Expect(err).To(HaveOccurred())
			})
			PIt("Should fail to get meta.Accessor", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

// setup returns a fakeClient for testing purposes
func setup(testNs *corev1.Namespace) client.Client {
	defaultFeatures := []string{"razee", "meterbase"}
	viper.Set("assets", "../../../assets")
	viper.Set("features", defaultFeatures)
	objs := []runtime.Object{
		testNs,
	}
	s := scheme.Scheme
	_ = monitoringv1.AddToScheme(s)

	client := fake.NewFakeClient(objs...)
	return client
}
