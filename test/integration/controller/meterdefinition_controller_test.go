package controller_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/onsi/gomega/gstruct"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"

	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
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

		PIt("Should find a meterdef", func(done Done) {
			Eventually(func() bool {
				result, _ := testHarness.Do(
					context.TODO(),
					GetAction(types.NamespacedName{Name: meterdef.Name, Namespace: Namespace}, meterdef),
				)

				utils.PrettyPrint(meterdef.Status)
				return result.Is(Continue)
			}, timeout, interval).Should(BeTrue(),"Find a meterdef and update status conditions")

			Eventually(func() (assertion bool) {
				assertion = runAssertionOnMeterDef(*meterdef)
				return assertion

			}, 300, interval).Should(BeTrue(),"Query prometheus and append metric data to status")
			close(done)
		}, 300)

		// FIt("Should query prom and append metric data to meterdef status", func(done Done) {
		// 	Eventually(func() (assertion bool) {
		// 		result, _ := testHarness.Do(
		// 			context.TODO(),
		// 			GetAction(types.NamespacedName{Name: meterdef.Name, Namespace: Namespace}, meterdef),
		// 		)

		// 		if !result.Is(Continue) {
		// 			return false
		// 		}

		// 		assertion = runAssertionOnMeterDef(*meterdef)
		// 		return assertion

		// 	}, 300, interval).Should(BeTrue())
		// 	close(done)
		// }, 300)
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
