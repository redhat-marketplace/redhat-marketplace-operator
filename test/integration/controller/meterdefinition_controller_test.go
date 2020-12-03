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

	// . "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MeterDefController reconcile", func() {
	BeforeEach(func() {
		Expect(testHarness.BeforeAll()).To(Succeed())
	})

	AfterEach(func() {
		Expect(testHarness.AfterAll()).To(Succeed())
	})

	Context("Meterdefinition reconcile", func() {

		var meterdef *v1alpha1.MeterDefinition
		certConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.OPERATOR_CERTS_CA_BUNDLE_NAME,
				Namespace: Namespace,
			},
		}

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

		FIt("Should find a meterdef", func(done Done) {
			Eventually(func() bool {
				err := testHarness.Get(context.TODO(), types.NamespacedName{Name: meterdef.Name, Namespace: Namespace}, meterdef)
				if err != nil {
					return false
				}

				return true
			}, timeout).Should(BeTrue())
			close(done)
		}, 180)

		It("Should query prometheus and append metric data to meterdef status", func(done Done) {
			Eventually(func() bool {
				err := testHarness.Get(context.TODO(), types.NamespacedName{Name: meterdef.Name, Namespace: Namespace}, meterdef)
				if err != nil {
					fmt.Println(err)
					return false
				}
				if err != nil {
					return false
				}

				assertion := runAssertionOnMeterDef(*meterdef)
				return assertion

			}, 300).Should(BeTrue())
			close(done)
		}, 300)

		Context("Error handling for operator-cert-ca-bundle config map", func() {
			BeforeEach(func(done Done) {
				Eventually(func() bool {
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: certConfigMap.Name, Namespace: certConfigMap.Namespace}, certConfigMap)
					if err != nil {
						return false
					}

					assertion := Expect(testHarness.Delete(context.TODO(), certConfigMap)).Should(Succeed())
					return assertion
				}, 300).Should(BeTrue())
				close(done)
			}, 300)

			It("Should log an error if the operator-cert-ca-bundle config map is not found", func(done Done) {
				Eventually(func() bool {
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: meterdef.Name, Namespace: Namespace}, meterdef)
					if err != nil {
						return false
					}

					assertion := runAssertionOnConditions(meterdef.Status, "Failed to retrieve operator-certs-ca-bundle.: ConfigMap \"operator-certs-ca-bundle\" not found")

					return assertion

				}, 300).Should(BeTrue())
				close(done)
			}, 300)
		})

		Context("Error handling for operator-cert-ca-bundle config map", func() {
			BeforeEach(func(done Done) {
				Eventually(func() bool {
					certConfigMap.Data = map[string]string{
						"wrong-key": "wrong-key",
					}

					assertion := Expect(testHarness.Upsert(context.TODO(), certConfigMap)).Should(Succeed())

					return assertion
				}, 300).Should(BeTrue())
				close(done)
			}, 300)

			It("Should log an error if the operator-cert-ca-bundle config map is misconfigured", func(done Done) {
				Eventually(func() bool {
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: meterdef.Name, Namespace: Namespace}, meterdef)
					if err != nil {
						return false
					}

					assertion := runAssertionOnConditions(meterdef.Status, "Error retrieving cert from config map")

					return assertion

				}, 300).Should(BeTrue())
				close(done)
			}, 300)
		})

		Context("Error handling for prometheus service", func() {
			BeforeEach(func(done Done) {
				promservice := corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhm-prometheus-meterbase",
						Namespace: Namespace,
					},
				}

				Expect(testHarness.Delete(context.TODO(), &promservice)).Should(Succeed())
				close(done)
			}, 300)

			It("Should log an error if the prometheus service isn't found", func(done Done) {
				Eventually(func() bool {
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: meterdef.Name, Namespace: meterdef.Namespace}, meterdef)
					if err != nil {
						return false
					}

					errorMsg := "failed to get prometheus service: Service \"rhm-prometheus-meterbase\" not found"
					assertion := runAssertionOnConditions(meterdef.Status, errorMsg)
					return assertion
				}, 300).Should(BeTrue())
				close(done)
			}, 300)
		})
	})
})

func runAssertionOnConditions(status v1alpha1.MeterDefinitionStatus, errorMsg string) (assertion bool) {
	for _, condition := range status.Conditions {
		if condition.Message == errorMsg {
			assertion = true
			return assertion
		}
	}

	assertion = false
	return assertion
}

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
				// depending on when this runs it could be 1 or 2
				"value": Not(BeZero()),
			}))
		}
	}

	return assertion
}
