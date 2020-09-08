// Copyright 2020 IBM Corp.
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
	const interval = time.Second * 5

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
				if !result.Is(Continue) {
					return nil
				}

				return meterreport.Status.AssociatedJob
			}, timeout, interval).Should(
				And(Not(BeNil()),
					WithTransform(func(o *common.JobReference) string {
						return o.Name
					}, Equal(job.Name))))

			updateJob := job.DeepCopy()
			updateJob.Status.Failed = 0
			updateJob.Status.Active = 0
			updateJob.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(context.Background(), updateJob)).Should(Succeed())

			Eventually(func() string {
				result, _ := cc.Do(
					context.Background(),
					GetAction(types.NamespacedName{Name: meterreport.Name, Namespace: namespace}, meterreport),
				)
				if !result.Is(Continue) {
					return ""
				}

				return meterreport.Status.Conditions.GetCondition(v1alpha1.ReportConditionTypeJobRunning).Message
			}, timeout, interval).Should(Equal(v1alpha1.ReportConditionJobFinished.Message))
		})
	})
})
