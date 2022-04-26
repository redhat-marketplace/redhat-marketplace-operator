// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engine

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/processors"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	testcase1 "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/test/engine_testcase1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const timeout = time.Second * 5
const heartBeat = time.Second

var _ = Describe("EngineTest", func() {
	var ctx context.Context
	var cancel context.CancelFunc
	var prometheusData *metrics.PrometheusData

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		var err error
		prometheusData = metrics.ProvidePrometheusData()
		engine, err = NewEngine(
			ctx,
			pkgtypes.Namespaces{""},
			scheme,
			managers.ClientOptions{
				Namespace:    "",
				DryRunClient: false,
			},
			logf.Log.WithName("engine"),
			prometheusData,
			processors.StatusFlushDuration(time.Second))
		Expect(err).ToNot(HaveOccurred())

		err = engine.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		By("stopping the engine")
		cancel()
	})

	It("should start up and monitor meter definitions and related objects", func() {
		Expect(k8sClient.Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ibm-common-services",
			},
		})).Should(Succeed())

		Expect(k8sClient.Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "openshift-redhat-marketplace",
			},
		})).Should(Succeed())

		Expect(k8sClient.Create(context.TODO(), testcase1.MDefExample)).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), testcase1.Pod)).Should(Succeed())

		Expect(k8sClient.Create(context.TODO(), testcase1.MdefChargeBack.DeepCopy())).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), testcase1.CSVLicensing.DeepCopy())).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), testcase1.ServiceInstance.DeepCopy())).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), testcase1.ServicePrometheus.DeepCopy())).Should(Succeed())

		By("checking the start state")
		Eventually(func() bool {
			_, ok, _ := prometheusData.Get("service").Get(testcase1.ServiceInstance)
			return ok
		}, timeout, heartBeat).Should(BeTrue(), "find service instance")

		Eventually(func() bool {
			_, ok, _ := prometheusData.Get("service").Get(testcase1.ServicePrometheus)
			return ok
		}, timeout, heartBeat).Should(BeTrue(), "find prometheus instance")

		Eventually(func() bool {
			_, ok, _ := prometheusData.Get("pod").Get(testcase1.Pod)
			return ok
		}, timeout, heartBeat).Should(BeTrue(), "find pod instance")

		By("deleting the service")
		Expect(k8sClient.Delete(context.TODO(), testcase1.ServiceInstance)).Should(Succeed())

		Eventually(func() bool {
			_, ok, _ := prometheusData.Get("service").Get(testcase1.ServiceInstance)
			return ok
		}, timeout, heartBeat).ShouldNot(BeTrue(), "not find service instance")

		Eventually(func() []common.WorkloadResource {
			meterDef := &marketplacev1beta1.MeterDefinition{}
			key := client.ObjectKeyFromObject(testcase1.MdefChargeBack)
			k8sClient.Get(context.TODO(), key, meterDef)
			return meterDef.Status.WorkloadResources
		}, timeout*2, heartBeat).Should(
			And(
				HaveLen(1),
				MatchAllElementsWithIndex(IndexIdentity, Elements{
					"0": MatchAllFields(Fields{
						"ReferencedWorkloadName": Equal(""),
						"NamespacedNameReference": MatchAllFields(Fields{
							"UID":       BeAssignableToTypeOf(types.UID("")),
							"Namespace": Equal("ibm-common-services"),
							"Name":      Equal("ibm-licensing-service-prometheus"),
							"GroupVersionKind": PointTo(MatchAllFields(Fields{
								"APIVersion": Equal("v1"),
								"Kind":       Equal("Service"),
							})),
						}),
					}),
				}),
			))

		By("recreating the service")
		Expect(k8sClient.Create(context.TODO(), testcase1.ServiceInstance)).Should(Succeed())

		Eventually(func() bool {
			_, ok, _ := prometheusData.Get("service").Get(testcase1.ServiceInstance)
			return ok
		}, timeout, heartBeat).Should(BeTrue(), "find service instance")

		By("deleting the meterdefinition")
		Expect(k8sClient.Delete(context.TODO(), testcase1.MdefChargeBack)).Should(Succeed())

		Eventually(func() bool {
			_, ok, _ := prometheusData.Get("service").Get(testcase1.ServiceInstance)
			return ok
		}, 2*timeout, heartBeat).ShouldNot(BeTrue(), "find service instance")

		Eventually(func() bool {
			_, ok, _ := prometheusData.Get("service").Get(testcase1.ServicePrometheus)
			return ok
		}, timeout, heartBeat).ShouldNot(BeTrue(), "find prometheus instance")

		By("recreating the meterdefinition")
		Expect(k8sClient.Create(context.TODO(), testcase1.MdefChargeBack.DeepCopy())).Should(Succeed())

		Eventually(func() bool {
			_, ok, _ := prometheusData.Get("service").Get(testcase1.ServiceInstance)
			return ok
		}, timeout, heartBeat).Should(BeTrue(), "find service instance")

		Eventually(func() bool {
			_, ok, _ := prometheusData.Get("service").Get(testcase1.ServicePrometheus)
			return ok
		}, timeout, heartBeat).Should(BeTrue(), "find prometheus instance")

		By("checking if the meter definition has the correct values")
		Eventually(func() []common.WorkloadResource {
			meterDef := &marketplacev1beta1.MeterDefinition{}
			key := client.ObjectKeyFromObject(testcase1.MdefChargeBack)
			k8sClient.Get(context.TODO(), key, meterDef)
			return meterDef.Status.WorkloadResources
		}, timeout, heartBeat).Should(
			And(
				HaveLen(2),
				MatchAllElementsWithIndex(IndexIdentity, Elements{
					"0": MatchAllFields(Fields{
						"ReferencedWorkloadName": Equal(""),
						"NamespacedNameReference": MatchAllFields(Fields{
							"UID":       BeAssignableToTypeOf(types.UID("")),
							"Namespace": Equal("ibm-common-services"),
							"Name":      Equal("ibm-licensing-service-instance"),
							"GroupVersionKind": PointTo(MatchAllFields(Fields{
								"APIVersion": Equal("v1"),
								"Kind":       Equal("Service"),
							})),
						}),
					}),
					"1": MatchAllFields(Fields{
						"ReferencedWorkloadName": Equal(""),
						"NamespacedNameReference": MatchAllFields(Fields{
							"UID":       BeAssignableToTypeOf(types.UID("")),
							"Namespace": Equal("ibm-common-services"),
							"Name":      Equal("ibm-licensing-service-prometheus"),
							"GroupVersionKind": PointTo(MatchAllFields(Fields{
								"APIVersion": Equal("v1"),
								"Kind":       Equal("Service"),
							})),
						}),
					}),
				}),
			))
	})
})
