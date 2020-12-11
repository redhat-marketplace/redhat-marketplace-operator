package controller_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/onsi/gomega/gstruct"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
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

		BeforeEach(func(done Done) {
			meterdef = &v1alpha1.MeterDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "meterdef-controller-test",
					Namespace: Namespace,
				},
				Spec: v1alpha1.MeterDefinitionSpec{
					Group:              "marketplace.redhat.com",
					Kind:               "MetricState",
					WorkloadVertexType: v1alpha1.WorkloadVertexOperatorGroup,
					Workloads: []v1alpha1.Workload{
						{
							Name:         "test",
							WorkloadType: v1alpha1.WorkloadTypePod,
							OwnerCRD: &common.GroupVersionKind{
								APIVersion: "apps/v1",
								Kind:       "StatefulSet",
							},
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "prometheus",
								},
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

			Eventually(func() bool {
				return Expect(testHarness.Create(context.TODO(), meterdef)).Should(SucceedOrAlreadyExist, "create the meterdef")
			}, 180).Should(BeTrue(), "Should create a meterdef if not found")

			// update the requeue rate of the meterdef controller
			Eventually(func() bool {
				assertion := updateOperatorDeploymentRequeueRate()
				return assertion
			}, 120).Should(BeTrue())

			Eventually(func() bool {
				certConfigMap := &corev1.ConfigMap{}
				promService := &corev1.Service{}

				assertion := Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: utils.OPERATOR_CERTS_CA_BUNDLE_NAME, Namespace: Namespace}, certConfigMap)).Should(Succeed(), "find config map")
				assertion = Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: utils.PROMETHEUS_METERBASE_NAME, Namespace: Namespace}, promService)).Should(Succeed(), "find prom service")
				return assertion
			}, 300).Should(BeTrue(), "check for config map and prom service")

			close(done)
		}, 400)

		AfterEach(func(done Done) {
			Expect(testHarness.Delete(context.TODO(), meterdef)).Should(Succeed())
			close(done)
		}, 120)

		It("Should find a meterdef", func(done Done) {
			Eventually(func() bool {
				assertion := Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: "meterdef-controller-test", Namespace: Namespace}, meterdef)).Should(Succeed(), "find a meterdef")
				return assertion
			}, 180).Should(BeTrue(), "should find a meterdef")
			close(done)
		}, 180)

		It("Should query prometheus and append metric data to meterdef status", func(done Done) {
			Eventually(func() bool {
				assertion := Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: "meterdef-controller-test", Namespace: Namespace}, meterdef)).Should(Succeed(), "find meterdef with metric data")
				assertion = runAssertionOnMeterDef(meterdef)
				return assertion
			}, 600).Should(BeTrue())
			close(done)
		}, 600)
	})
})

func runAssertionOnMeterDef(meterdef *v1alpha1.MeterDefinition) (assertion bool) {
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
				"value":        Not(BeZero()),
			}))
		}
	}

	return assertion
}

func updateOperatorDeploymentRequeueRate() bool {
	rhmDeployment := &appsv1.Deployment{}

	assertion := Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: "redhat-marketplace-operator", Namespace: Namespace}, rhmDeployment)).Should(Succeed(), "Get rhm deployment")

	var containerIndex int
	for index, container := range rhmDeployment.Spec.Template.Spec.Containers {
		if container.Name == "redhat-marketplace-operator" {
			containerIndex = index
		}
	}

	rhmContiner := rhmDeployment.Spec.Template.Spec.Containers[containerIndex]

	var envIndex int
	for index, env := range rhmContiner.Env {
		if env.Name == "METER_DEF_CONTROLLER_REQUEUE_RATE" {
			envIndex = index
		}
	}

	rhmDeployment.Spec.Template.Spec.Containers[containerIndex].Env[envIndex].Value = "25"

	assertion = Expect(testHarness.Update(context.TODO(), rhmDeployment)).Should(Succeed(), "update deployment")
	return assertion
}
