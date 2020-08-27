package testenv

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MeterReportController", func() {
	const timeout = time.Second * 30
	const interval = time.Second * 1

	Context("MeterReport reconcile", func() {
		var (
			meterreport *v1alpha1.MeterReport
			start       time.Time
			end         time.Time
		)

		BeforeEach(func() {
			start, end = time.Now(), time.Now()

			start.Add(-72 * time.Hour)
			end.Add(-48 * time.Hour)

			meterreport = &v1alpha1.MeterReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "openshift-redhat-marketplace",
				},
				Spec: v1alpha1.MeterReportSpec{
					StartTime: metav1.NewTime(start),
					EndTime:   metav1.NewTime(end),
					PrometheusService: &common.ServiceReference{
						Namespace: namespace,
						Name:      "test",
					},
					MeterDefinitions: []v1alpha1.MeterDefinition{},
				},
			}
		})

		It("should create a job if the report is due", func() {
			Expect(k8sClient.Create(context.Background(), meterreport)).Should(Succeed())

			job := &batchv1.Job{}

			Eventually(func() bool {
				result, _ := cc.Do(
					context.Background(),
					GetAction(types.NamespacedName{Name: meterreport.Name, Namespace: namespace}, job),
				)
				return result.Is(Continue)
			}, timeout, interval).Should(BeTrue())
	
			Eventually(func() *common.JobReference {
				result, _ := cc.Do(
					context.Background(),
					GetAction(types.NamespacedName{Name: meterreport.Name, Namespace: namespace}, meterreport),
				)
				if !result.Is(Continue){
					return nil
				}

				return meterreport.Status.AssociatedJob
			}, timeout, interval).Should(
				And(Not(BeNil()),
					WithTransform(func(o *common.JobReference) string {
						return o.Name
					}, Equal(job.Name))))
		})
	})
})
