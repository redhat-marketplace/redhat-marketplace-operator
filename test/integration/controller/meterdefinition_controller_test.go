package controller_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/onsi/gomega/gstruct"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = FDescribe("MeterDefController reconcile", func() {
	BeforeEach(func() {
		Expect(testHarness.BeforeAll()).To(Succeed())
	})

	AfterEach(func() {
		Expect(testHarness.AfterAll()).To(Succeed())
	})

	Context("Meterdefinition reconcile", func() {

		var meterdef *v1alpha1.MeterDefinition

		BeforeEach(func(done Done) {
			meterdef = &v1alpha1.MeterDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-meterdef",
					Namespace: Namespace,
				},
				Spec: v1alpha1.MeterDefinitionSpec{
					Group:              "testgroup",
					Kind:               "testkind",
					WorkloadVertexType: v1alpha1.WorkloadVertexOperatorGroup,
					Workloads: []v1alpha1.Workload{
						{
							Name:         "test",
							WorkloadType: v1alpha1.WorkloadTypePod,
							OwnerCRD: &common.GroupVersionKind{
								APIVersion: "marketplace.redhat.com/v1alpha1",
								Kind:       "MeterBase",
							},
							MetricLabels: []v1alpha1.MeterLabelQuery{
								{
									Aggregation: "sum",
									Label:       "test",
									Query:       "kube_pod_info",
								},
							},
						},
					},
				},
			}

			Expect(testHarness.Create(context.TODO(), meterdef)).Should(SucceedOrAlreadyExist)
			close(done)
		}, 120)

		It("Should find a meterdef", func(done Done) {
			Eventually(func() bool {
				result, _ := testHarness.Do(
					context.TODO(),
					GetAction(types.NamespacedName{Name: meterdef.Name, Namespace: Namespace}, meterdef),
				)

				return result.Is(Continue)
			}, timeout, interval).Should(BeTrue())
			close(done)
		}, 180)

		It("Should query prometheus and append metric data to meterdef status", func(done Done) {
			Eventually(func() (assertion bool) {

				assertion = runAssertionOnMeterDef(*meterdef)
				return assertion

			}, 300, interval).Should(BeTrue())
			close(done)
		}, 300)

		When("There is a misssing or misconfigured prerequisite object the meterdef controller should log an error to conditions", func() {
			foundCertConfigMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      utils.OPERATOR_CERTS_CA_BUNDLE_NAME,
					Namespace: Namespace,
				},
			}

			Context("cert config map is missing", func() {
				BeforeEach(func(done Done) {
					Eventually(func() (assertion bool) {
						testHarness.Get(context.TODO(), types.NamespacedName{Name: foundCertConfigMap.Name, Namespace: foundCertConfigMap.Namespace}, &foundCertConfigMap)
						assertion = Expect(testHarness.Delete(context.TODO(), &foundCertConfigMap)).Should(Succeed())
						return assertion
					}, 300).Should(BeTrue())
					close(done)
				}, 300)

				It("Should log an error if the config map is not found", func(done Done) {
					Eventually(func() (assertion bool) {
						result, _ := testHarness.Do(
							context.TODO(),
							GetAction(types.NamespacedName{Name: meterdef.Name, Namespace: Namespace}, meterdef),
						)

						if !result.Is(Continue) {
							return false
						}

						for _, condition := range meterdef.Status.Conditions {
							if condition.Message == "Failed to retrieve operator-certs-ca-bundle.: ConfigMap \"operator-certs-ca-bundle\" not found" {
								assertion = true
							}
						}

						return assertion

					}, 300, interval).Should(BeTrue())
					close(done)
				}, 300)
			})

			Context("cert config map is misconfigured", func() {
				BeforeEach(func(done Done) {
					Eventually(func() (assertion bool) {
						err := testHarness.Get(context.TODO(), types.NamespacedName{Name: utils.OPERATOR_CERTS_CA_BUNDLE_NAME, Namespace: Namespace}, foundCertConfigMap.DeepCopyObject())
						if err != nil {
							fmt.Println("error retrieving", err)
							assertion = false
							return assertion
						}

						foundCertConfigMap.Data = map[string]string{
							"wrong-key": "wrong-key",
						}

						err = testHarness.Update(context.TODO(), foundCertConfigMap.DeepCopyObject())
						if err != nil {
							fmt.Println("error updating", err)
							assertion = false
							return assertion
						}

						assertion = true
						return assertion
					}, 300).Should(BeTrue())
					close(done)
				}, 300)

				It("Should log an error if the config map is misconfigured", func(done Done) {
					Eventually(func() (assertion bool) {
						err := testHarness.Get(context.TODO(), types.NamespacedName{Name: meterdef.Name, Namespace: Namespace}, meterdef)
						if err != nil {
							assertion = false
							return assertion
						}
						utils.PrettyPrintWithLog("Meterdef Status Error", meterdef.Status.Conditions)

						for _, condition := range meterdef.Status.Conditions {
							if condition.Message == "Error retrieving cert from config map" {
								assertion = true
								return assertion
							}
						}

						return assertion

					}, 300, interval).Should(BeTrue())
					close(done)
				}, 300)
			})
		})

	})
})

func runAssertionOnMeterDef(meterdef v1alpha1.MeterDefinition) (assertion bool) {
	if meterdef.Status.Results != nil {
		if meterdef.Status.Results[0].Value > 0 {
			startTime := meterdef.Status.Results[0].StartTime
			endTime := meterdef.Status.Results[0].EndTime

			result := map[string]interface{}{
				"value":        meterdef.Status.Results[0].Value,
				"endTime":      meterdef.Status.Results[0].EndTime,
				"startTime":    meterdef.Status.Results[0].StartTime,
				"queryName":    meterdef.Status.Results[0].QueryName,
				"workloadName": meterdef.Status.Results[0].WorkloadName,
			}

			assertion = Expect(result).Should(MatchAllKeys(Keys{
				"workloadName": Equal("test"),
				"queryName":    Equal("test"),
				"startTime":    Equal(startTime),
				"endTime":      Equal(endTime),
				"value":        Equal(int32(1)),
			}))
		}
	}

	return assertion
}
