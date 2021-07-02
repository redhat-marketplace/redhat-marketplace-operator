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

package controller_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("MeterReportController", func() {
	Context("MeterReport reconcile", func() {
		var (
			meterreport *v1alpha1.MeterReport
			meterdef    *v1alpha1.MeterDefinition
			start       time.Time
			end         time.Time
		)

		BeforeEach(func() {

			start, end = time.Now(), time.Now()
			start = start.Add(-5 * time.Minute)

			meterdef = &v1alpha1.MeterDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
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
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": "rhm-metric-state",
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

			meterreport = &v1alpha1.MeterReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "openshift-redhat-marketplace",
				},
				Spec: v1alpha1.MeterReportSpec{
					StartTime: metav1.NewTime(start),
					EndTime:   metav1.NewTime(end),
					PrometheusService: &common.ServiceReference{
						Namespace:  Namespace,
						Name:       "rhm-prometheus-meterbase",
						TargetPort: intstr.FromString("rbac"),
					},
					ExtraArgs: []string{
						"--uploadTargets", "noop",
					},
					MeterDefinitions: []v1alpha1.MeterDefinition{
						*meterdef,
					},
				},
			}

			Expect(testHarness.Create(context.TODO(), meterdef)).Should(SucceedOrAlreadyExist)
		})

		AfterEach(func() {
			testHarness.Delete(context.TODO(), meterdef)
			testHarness.Delete(context.TODO(), meterreport)
		})

		It("should create a job if the report is due", func() {
			Expect(testHarness.Create(context.TODO(), meterreport)).Should(Succeed())
			job := &batchv1.Job{}

			Eventually(func() bool {
				result, _ := testHarness.Do(
					context.TODO(),
					GetAction(types.NamespacedName{Name: meterreport.Name, Namespace: Namespace}, job),
				)
				return result.Is(Continue)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() map[string]interface{} {
				result, _ := testHarness.Do(
					context.TODO(),
					GetAction(types.NamespacedName{Name: meterreport.Name, Namespace: Namespace}, meterreport),
				)

				if !result.Is(Continue) {
					return map[string]interface{}{
						"result": result.Status,
					}
				}

				return map[string]interface{}{
					"result": result.Status,
					"job":    meterreport.Status.AssociatedJob,
				}
			}, timeout, interval).Should(
				MatchAllKeys(Keys{
					"result": Equal(Continue),
					"job": WithTransform(func(o *common.JobReference) string {
						if o == nil {
							return ""
						}
						return o.Name
					}, Equal(job.Name)),
				}))

			Eventually(func() map[string]interface{} {
				result, _ := testHarness.Do(
					context.TODO(),
					GetAction(types.NamespacedName{Name: meterreport.Name, Namespace: Namespace}, meterreport),
				)
				if !result.Is(Continue) {
					return map[string]interface{}{
						"resultStatus": result.Status,
					}
				}

				if meterreport.Status.Conditions == nil {
					return map[string]interface{}{
						"resultStatus": "noConditions",
					}
				}

				cond := meterreport.Status.Conditions.GetCondition(v1alpha1.ReportConditionTypeJobRunning)
				return map[string]interface{}{
					"resultStatus":     result.Status,
					"conditionType":    cond.Type,
					"conditionMessage": cond.Message,
					"conditionStatus":  cond.Status,
				}
			}, timeout, interval).Should(
				MatchAllKeys(Keys{
					"resultStatus":     Equal(Continue),
					"conditionType":    Equal(v1alpha1.ReportConditionJobFinished.Type),
					"conditionMessage": Equal(v1alpha1.ReportConditionJobFinished.Message),
					"conditionStatus":  Equal(v1alpha1.ReportConditionJobFinished.Status),
				}))
		})

		It("should upload to the data service if the upload target is set", func() {
			Eventually(func() bool {
				meterbase := &v1alpha1.MeterBase{}
		
				err := testHarness.Get(context.TODO(),types.NamespacedName{Name: "rhm-marketplaceconfig-meterbase",Namespace: "openshift-redhat-marketplace"} ,meterbase)
				if err != nil {
					return false
				}

				newMeterBase := meterbase.DeepCopy()
		
				newMeterBase.Spec.DataServiceEnabled = true
		
				err = testHarness.Update(context.TODO(),newMeterBase)			
				if err != nil {
					return false
				}

				return true		
			
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				ss := &appsv1.StatefulSet{}
				err := testHarness.Get(context.TODO(),types.NamespacedName{Name: "rhm-dqlite",Namespace: "openshift-redhat-marketplace"} ,ss)
				if err != nil {
					return false
				}

				if ss.Status.ReadyReplicas == 3 {
					return true
				}

				return false
			},dataServiceTimeout, interval).Should(BeTrue())
			job := &batchv1.Job{}
			Eventually(func() bool {
				meterreport.Spec.ExtraArgs = []string{"--uploadTarget=data-service"}
				result, err := testHarness.Do(
					context.TODO(),
					CreateAction(meterreport),
				)

				if err !=nil {
					fmt.Printf("error creating meterreport %s",err)
				}

				result, _ = testHarness.Do(
					context.TODO(),
					GetAction(types.NamespacedName{Name: meterreport.Name, Namespace: Namespace}, job),
				)

				return result.Is(Continue)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() map[string]interface{} {
				result, _ := testHarness.Do(
					context.TODO(),
					GetAction(types.NamespacedName{Name: meterreport.Name, Namespace: Namespace}, meterreport),
				)

				if !result.Is(Continue) {
					return map[string]interface{}{
						"result": result.Status,
					}
				}

				return map[string]interface{}{
					"result": result.Status,
					"job":    meterreport.Status.AssociatedJob,
				}
			}, timeout, interval).Should(
				MatchAllKeys(Keys{
					"result": Equal(Continue),
					"job": WithTransform(func(o *common.JobReference) string {
						if o == nil {
							return ""
						}
						return o.Name
					}, Equal(job.Name)),
				}))

			Eventually(func() map[string]interface{} {
				result, _ := testHarness.Do(
					context.TODO(),
					GetAction(types.NamespacedName{Name: meterreport.Name, Namespace: Namespace}, meterreport),
				)
				if !result.Is(Continue) {
					return map[string]interface{}{
						"resultStatus": result.Status,
					}
				}

				if meterreport.Status.Conditions == nil {
					return map[string]interface{}{
						"resultStatus": "noConditions",
					}
				}

				cond := meterreport.Status.Conditions.GetCondition(v1alpha1.ReportConditionTypeJobRunning)
				return map[string]interface{}{
					"resultStatus":     result.Status,
					"conditionType":    cond.Type,
					"conditionMessage": cond.Message,
					"conditionStatus":  cond.Status,
				}
			}, jobTimeout, interval).Should(
				MatchAllKeys(Keys{
					"resultStatus":     Equal(Continue),
					"conditionType":    Equal(v1alpha1.ReportConditionJobFinished.Type),
					"conditionMessage": Equal(v1alpha1.ReportConditionJobFinished.Message),
					"conditionStatus":  Equal(v1alpha1.ReportConditionJobFinished.Status),
				}))
		})
	})
})
