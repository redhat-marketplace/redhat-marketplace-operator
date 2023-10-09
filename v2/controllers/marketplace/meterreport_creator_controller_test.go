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

package marketplace

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MeterbaseController", func() {
	Describe("check date functions", func() {
		var (
			ctrl *MeterReportCreatorReconciler
		)

		meterDef1 := &marketplacev1beta1.MeterDefinition{}
		meterDef2 := &marketplacev1beta1.MeterDefinition{}
		meterDef3 := &marketplacev1beta1.MeterDefinition{}
		materDefNoLabel := &marketplacev1beta1.MeterDefinition{}
		catNameA := "labelA"
		mA := make(map[string]string)
		mA["marketplace.redhat.com/category"] = catNameA
		catNameB := "labelB"
		mB := make(map[string]string)
		mB["marketplace.redhat.com/category"] = catNameB

		meterDef1.SetLabels(mA)
		meterDef2.SetLabels(mB)
		meterDef3.SetLabels(mA)
		meterDefinitionList := []marketplacev1beta1.MeterDefinition{*meterDef1, *meterDef2, *meterDef3, *materDefNoLabel}

		endDate := time.Now().UTC()
		endDate = endDate.AddDate(0, 0, 0)
		minDate := endDate.AddDate(0, 0, 0)

		BeforeEach(func() {
			ctrl = &MeterReportCreatorReconciler{}
		})

		It("reports should calculate the correct dates to create", func() {

			exp := ctrl.generateExpectedDates(endDate, time.UTC, -30, minDate)
			Expect(exp).To(HaveLen(1))

			minDate = endDate.AddDate(0, 0, -2)

			exp = ctrl.generateExpectedDates(endDate, time.UTC, -30, minDate)
			Expect(exp).To(HaveLen(3))
		})

		It("should return category names, excluding duplicated", func() {
			Expect(meterDefinitionList).To(HaveLen((4)))
			categoryList := getCategoriesFromMeterDefinitions(meterDefinitionList)
			Expect(categoryList).To(HaveLen(3))
			// if meter definition has not label set up, category name is saved as an empty string ("") into category list
			Expect(categoryList).To(ContainElements([]string{catNameA, catNameB, ""}))
		})

		It("should put category name into newly created meter report name", func() {
			endDate := time.Date(2021, time.June, 1, 0, 0, 0, 0, time.UTC)
			minDate := endDate.AddDate(0, 0, 0)
			exp := ctrl.generateExpectedDates(endDate, time.UTC, -30, minDate)
			nameFromString := ctrl.newMeterReportNameFromString(exp[0])
			Expect(nameFromString).To(Equal("meter-report-2021-06-01"))
		})

		It("should retrieve date properly for report name for old and new format (with and without category)", func() {
			endDate := time.Date(2021, time.June, 1, 0, 0, 0, 0, time.UTC)
			// old report name: meter-report-[date]
			foundTime, _ := ctrl.retrieveCreatedDate("meter-report-2021-06-01")
			Expect(foundTime).To(Equal(endDate))
		})

		It("should return only non-duplicated categories", func() {
			categoryList := getCategoriesFromMeterDefinitions(meterDefinitionList)
			Expect(categoryList).To(HaveLen(3))
			Expect(categoryList).To(ContainElements([]string{catNameA, catNameB, ""}))
		})
	})

	Describe("check reconciller", func() {
		var (
			name             = utils.METERBASE_NAME
			namespace        = operatorNamespace
			createdMeterBase *marketplacev1alpha1.MeterBase
			now              = time.Now().UTC()
		)

		key := types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}

		BeforeEach(func() {
			operatorCfg.ReportController.PollTime = 10 * time.Second
			reportCreatorReconciler := &MeterReportCreatorReconciler{
				Log:    ctrl.Log.WithName("controllers").WithName("MeterReportCreator"),
				Client: k8sClient,
				Scheme: k8sScheme,
				Cfg:    operatorCfg,
			}
			err := reportCreatorReconciler.SetupWithManager(k8sManager, doneChan)
			Expect(err).ToNot(HaveOccurred())

			k8sClient.Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name: operatorNamespace,
			}})

			createdMeterBase = &marketplacev1alpha1.MeterBase{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
				Spec: marketplacev1alpha1.MeterBaseSpec{
					Enabled: true,
				},
			}

			Expect(k8sClient.Create(context.TODO(), createdMeterBase)).Should(Succeed())

			createdMeterBase.Status = marketplacev1alpha1.MeterBaseStatus{
				Conditions: status.Conditions{
					status.Condition{
						Type:               marketplacev1alpha1.ConditionInstalling,
						Status:             corev1.ConditionFalse,
						Reason:             marketplacev1alpha1.ReasonMeterBaseFinishInstall,
						Message:            "finished install",
						LastTransitionTime: metav1.Now(),
					},
				},
			}

			createDate := metav1.Now().Add(-5 * 24 * time.Hour)
			createdMeterBase.CreationTimestamp = metav1.NewTime(createDate)

			Expect(k8sClient.Status().Update(context.TODO(), createdMeterBase)).Should(Succeed())
		})

		AfterEach(func() {
			// Add any teardown steps that needs to be executed after each test
			Expect(k8sClient.Delete(context.TODO(), createdMeterBase)).Should(Succeed())
		})

		It("should run meterbase creator reconciler", func() {
			k8sClient.Get(context.TODO(), key, createdMeterBase)
			By("Expecting status")
			Eventually(func() *status.Condition {
				f := &marketplacev1alpha1.MeterBase{}
				k8sClient.Get(context.TODO(), key, f)
				return f.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionInstalling)
			}, timeout, interval).Should(PointTo(MatchFields(IgnoreExtras, Fields{
				"Status": Equal(corev1.ConditionFalse),
				"Reason": Equal(marketplacev1alpha1.ReasonMeterBaseFinishInstall),
			})))

			Eventually(func() map[string]interface{} {
				meterReportList := &marketplacev1alpha1.MeterReportList{}
				err := k8sClient.List(context.TODO(), meterReportList, client.InNamespace(operatorNamespace))
				Expect(err).To(Not(HaveOccurred()))

				meterReportNames := []string{}
				for _, report := range meterReportList.Items {
					meterReportNames = append(meterReportNames, report.Name)
				}

				return map[string]interface{}{
					"items":       meterReportList.Items,
					"reportNames": meterReportNames,
				}
			}, timeout, interval).Should(MatchAllKeys(Keys{
				"items":       HaveLen(1),
				"reportNames": ContainElements("meter-report-" + now.Format(utils.DATE_FORMAT)),
			}))
		})
	})
})
