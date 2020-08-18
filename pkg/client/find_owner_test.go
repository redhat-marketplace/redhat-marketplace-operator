package client_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	rhmClient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var _ = Describe("FindOwner", func() {

	var (
		ownerHelper *rhmClient.FindOwnerHelper
		inClient    dynamic.Interface
		restMapper  meta.RESTMapper
		restConfig  *rest.Config
		testNs      *corev1.Namespace
		cl          client.Client
	)

	BeforeEach(func() {

		restConfig, _ = config.GetConfig()
		inClient, _ = dynamic.NewForConfig(restConfig)
		restMapper, _ = managers.NewDynamicRESTMapper(restConfig)
		cl = setup(restConfig)
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
				Expect(ownerHelper.GetRestMapper()).ToNot(BeNil())
			})
		})
		Context("Get Client", func() {
			It("Should be the same as original dynamic.Interface", func() {
				Expect(ownerHelper.GetClient()).To(Equal(inClient))
				Expect(ownerHelper.GetClient()).ToNot(BeNil())
			})
		})
	})

	Describe("FindOwner function test", func() {
		var (
			err              error
			meterdef         *marketplacev1alpha1.MeterDefinition
			owner            *metav1.OwnerReference
			kind, apiVersion string
			name, namespace  string
			pod              *corev1.Pod
		)
		BeforeEach(func() {

			name = "test-meterdef"
			namespace = "test-ns"

			testNs = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}
			wl := marketplacev1alpha1.Workload{
				Name:         "podcpu",
				WorkloadType: "Pod",
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
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			}
			owner = metav1.NewControllerRef(pod, meterdef.GroupVersionKind())

		})
		Context("When the function succeeeds, and the OwnerReference is returned", func() {
			It("Should return the OwnerReference", func() {
				err = cl.Create(context.TODO(), testNs)
				if err != nil && !errors.IsAlreadyExists(err) {
					Fail("Cant Create Namespace: " + err.Error())
				}
				err = cl.Create(context.TODO(), pod)
				if err != nil && !errors.IsAlreadyExists(err) {
					Fail("Cant Create pod: " + err.Error())
				}
				podret := &corev1.Pod{}
				err = cl.Get(context.TODO(), types.NamespacedName{Name: "pod", Namespace: namespace}, podret)
				if err != nil {
					Fail("Could not get pod: " + err.Error())
				}
				controller := true
				blockownerdeletion := false
				meterdef.OwnerReferences = []metav1.OwnerReference{
					{
						APIVersion:         "v1",
						Kind:               "Pod",
						Name:               "pod",
						UID:                podret.UID,
						Controller:         &controller,
						BlockOwnerDeletion: &blockownerdeletion,
					},
				}
				err = cl.Create(context.TODO(), meterdef)
				if err != nil && !errors.IsAlreadyExists(err) {
					Fail("Cant Create MeterDef: " + err.Error())
				}
				time.Sleep(7 * time.Second)
				owner, err = ownerHelper.FindOwner(name, namespace, owner)
				Expect(err).To(BeNil())
				Expect(owner.Name).To(Equal("pod"))
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
			It("Should fail to get resources", func() {
				owner, err = ownerHelper.FindOwner("wrongName", "worngNamespace", owner)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

// setup returns a client for testing purposes
func setup(restConfig *rest.Config) client.Client {
	defaultFeatures := []string{"razee", "meterbase"}
	viper.Set("assets", "../../../assets")
	viper.Set("features", defaultFeatures)
	s := scheme.Scheme
	_ = monitoringv1.AddToScheme(s)
	_ = marketplacev1alpha1.AddToScheme(s)

	cl, err := client.New(restConfig, client.Options{})
	if err != nil {
		Fail("failed to create client")
	}
	return cl
}
