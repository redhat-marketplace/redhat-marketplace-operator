package client_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	rhmClient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var _ = Describe("FindOwner", func() {

	var (
		cache cache.Cache
		err   error
		objs  []runtime.Object
	)

	BeforeEach(func() {
	})

	Describe("Testing GVK_Indexer File, for successful cases", func() {

		BeforeEach(func() {
			restConfig, err := config.GetConfig()
			if err != nil {
				Fail("Could not get rest.config" + err.Error())
			}
			restMapper, err := managers.NewDynamicRESTMapper(restConfig)
			if err != nil {
				Fail("Could not get DynamicRESTMapper" + err.Error())
			}
			opsSrcSchemeDefinition := controller.ProvideOpsSrcScheme()
			monitoringSchemeDefinition := controller.ProvideMonitoringScheme()
			olmV1SchemeDefinition := controller.ProvideOLMV1Scheme()
			olmV1Alpha1SchemeDefinition := controller.ProvideOLMV1Alpha1Scheme()
			openshiftConfigV1SchemeDefinition := controller.ProvideOpenshiftConfigV1Scheme()
			localSchemes := controller.ProvideLocalSchemes(opsSrcSchemeDefinition, monitoringSchemeDefinition, olmV1SchemeDefinition, olmV1Alpha1SchemeDefinition, openshiftConfigV1SchemeDefinition)
			scheme, err := managers.ProvideScheme(restConfig, localSchemes)
			if err != nil {
				Fail("Could not get scheme" + err.Error())
			}
			clientOptions := managers.ClientOptions{
				Namespace:    "",
				DryRunClient: false,
			}
			cache, err = managers.ProvideNewCache(restConfig, restMapper, scheme, clientOptions)
			context.Background()
		})

		Context("AddOwningControllerIndex function test", func() {
			BeforeEach(func() {
				objs = []runtime.Object{
					&corev1.Pod{},
					&corev1.Service{},
					&corev1.PersistentVolumeClaim{},
					&monitoringv1.ServiceMonitor{},
				}
			})
			It("Should successfully add Owning Controller Index", func() {
				err = rhmClient.AddOwningControllerIndex(cache, objs)
				Expect(err).To(BeNil())
			})
		})
		Context("AddUIDIndex function test", func() {
			BeforeEach(func() {
				objs = []runtime.Object{
					&corev1.Pod{},
					&corev1.Service{},
					&corev1.PersistentVolumeClaim{},
					&marketplacev1alpha1.MeterDefinition{},
					&monitoringv1.ServiceMonitor{},
				}
			})
			It("Should successfully add UID Index", func() {
				err = rhmClient.AddUIDIndex(cache, objs)
				Expect(err).To(BeNil())
			})
		})
		Context("AddOperatorSourceIndex function test", func() {
			It("Should successfully AddOperatorSourceIndex", func() {
				err = rhmClient.AddOperatorSourceIndex(cache)
				Expect(err).To(BeNil())
			})
		})
		Context("AddAnnotationIndex function test", func() {
			BeforeEach(func() {
				objs = []runtime.Object{
					&corev1.Pod{},
					&corev1.Service{},
					&olmv1alpha1.ClusterServiceVersion{},
					&marketplacev1alpha1.MeterDefinition{},
					&monitoringv1.ServiceMonitor{},
				}
			})
			It("Should successfully add Annotation Index", func() {
				err = rhmClient.AddAnnotationIndex(cache, objs)
				Expect(err).To(BeNil())
			})
		})
		Context("ObjRefToStr function test", func() {
			It("Should successfully return combined APIVersion and GVK", func() {
				apiVersion := "marketplace/v1alpha1"
				kind := "MeterDefinition"
				res := rhmClient.ObjRefToStr(apiVersion, kind)
				Expect(res).To(Equal("meterdefinition.v1alpha1.marketplace"))
			})
		})
	})
})
