package controller_test

import (
	"context"

	"fmt"

	"github.com/gotidy/ptr"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	osappsv1 "github.com/openshift/api/apps/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	corev1 "k8s.io/api/core/v1"
	ks8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = FDescribe("MeterDefInstallController reconcile", func() {
	Context("Meterdefinition reconcile", func() {

		var memcachedSub *olmv1alpha1.Subscription
		var catalogSource *olmv1alpha1.CatalogSource

		BeforeEach(func() {
			memcachedSub = &olmv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "memcached-subscription",
					Namespace: "openshift-redhat-marketplace",
					Labels: map[string]string{
						"marketplace.redhat.com/operator": "true",
					},
				},

				Spec: &olmv1alpha1.SubscriptionSpec{
					Channel:                "alpha",
					InstallPlanApproval:    olmv1alpha1.ApprovalManual,
					Package:                "memcached-operator-rhmp",
					CatalogSource:          "redhat-marketplace",
					CatalogSourceNamespace: "openshift-redhat-marketplace",
				},
			}

			catalogSource = &olmv1alpha1.CatalogSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redhat-marketplace",
					Namespace: "openshift-redhat-marketplace",
				},
				Spec: olmv1alpha1.CatalogSourceSpec{
					SourceType: olmv1alpha1.SourceType(olmv1alpha1.SourceTypeGrpc),
					Image:      "quay.io/mxpaspa/memcached-ansible-index:1.0.1",
				},
			}

			subSectionMeterBase := &marketplacev1alpha1.MeterBase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      utils.METERBASE_NAME,
					Namespace: Namespace,
				},
				Spec: marketplacev1alpha1.MeterBaseSpec{
					Enabled: false,
					Prometheus: &marketplacev1alpha1.PrometheusSpec{
						Storage: marketplacev1alpha1.StorageSpec{
							Size: resource.MustParse("30Gi"),
						},
						Replicas: ptr.Int32(2),
					},
					MeterdefinitionCatalogServer: &common.MeterDefinitionCatalogServer{
						SyncCommunityMeterDefinitions: ptr.Bool(true),
						SyncSystemMeterDefinitions: ptr.Bool(true),
						DeployMeterDefinitionCatalogServer: ptr.Bool(true),
					},
				},
			}

			meterBase := &marketplacev1alpha1.MeterBase{}
			err := testHarness.Get(context.TODO(),types.NamespacedName{Name: utils.METERBASE_NAME,Namespace: Namespace},meterBase)
			if ks8serrors.IsNotFound(err){
				Expect(testHarness.Create(context.TODO(),subSectionMeterBase)).Should(Succeed(),"create meterbase if not found")
			}

			if meterBase.Spec.MeterdefinitionCatalogServer == nil {
				meterBase.Spec.MeterdefinitionCatalogServer.SyncCommunityMeterDefinitions = ptr.Bool(true)
				meterBase.Spec.MeterdefinitionCatalogServer.SyncSystemMeterDefinitions = ptr.Bool(true)
				meterBase.Spec.MeterdefinitionCatalogServer.DeployMeterDefinitionCatalogServer = ptr.Bool(true)
				Expect(testHarness.Update(context.TODO(),meterBase)).Should(Succeed(),"set MeterdefinitionCatalogServer values if nil")
			}

			meterBase.Spec.MeterdefinitionCatalogServer = &common.MeterDefinitionCatalogServer{
				DeployMeterDefinitionCatalogServer: ptr.Bool(true),
				SyncCommunityMeterDefinitions: ptr.Bool(true),
				SyncSystemMeterDefinitions: ptr.Bool(true),
			}

			Expect(testHarness.Update(context.TODO(),meterBase)).Should(Succeed(),"set all MeterdefinitionCatalogServer values to true")

			Eventually(func() bool {
				dc := &osappsv1.DeploymentConfig{}
				err := testHarness.Get(context.TODO(), types.NamespacedName{Name: utils.DeploymentConfigName, Namespace: Namespace}, dc)
				if err != nil {
					fmt.Println(err.Error())
					return false
				}
			
				for _, c := range dc.Status.Conditions {
					if c.Type == osappsv1.DeploymentAvailable {
						if c.Status != corev1.ConditionTrue {
							return false
						}
					}
				}
			
				return true
			},timeout, interval).Should(BeTrue(),"deploymentconfig should be running")
			
			Expect(testHarness.Create(context.TODO(), memcachedSub)).Should(SucceedOrAlreadyExist, "create the memcached subscription")
			Expect(testHarness.Create(context.TODO(), catalogSource)).Should(SucceedOrAlreadyExist, "create the test catalog")
		})

		AfterEach(func() {
			testHarness.Get(context.TODO(), types.NamespacedName{Name: "redhat-marketplace", Namespace: "openshift-redhat-marketplace"}, catalogSource)
			testHarness.Delete(context.TODO(), catalogSource)

			memcachedSub := &olmv1alpha1.Subscription{}
			testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-subscription", Namespace: "openshift-redhat-marketplace"}, memcachedSub)
			testHarness.Delete(context.TODO(), memcachedSub)

			memcachedCSV := &olmv1alpha1.ClusterServiceVersion{}
			testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-operator.v0.0.1", Namespace: "openshift-redhat-marketplace"}, memcachedCSV)
			testHarness.Delete(context.TODO(), memcachedCSV)

			testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-operator.v0.0.2", Namespace: "openshift-redhat-marketplace"}, memcachedCSV)
			testHarness.Delete(context.TODO(), memcachedCSV)
		})

		Context("memcached 0.0.1", func() {
			It("Should create meterdefs for memcached 0.0.1", func() {
				Eventually(func() []string {
					foundSub := &olmv1alpha1.Subscription{}
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-subscription", Namespace: "openshift-redhat-marketplace"}, foundSub)
					if err != nil {
						return nil
					}

					if foundSub.Status.InstallPlanRef == nil {
						return nil
					}

					installPlanName := foundSub.Status.InstallPlanRef.Name
					foundInstallPlan := &olmv1alpha1.InstallPlan{}
					err = testHarness.Get(context.TODO(), types.NamespacedName{Name: installPlanName, Namespace: "openshift-redhat-marketplace"}, foundInstallPlan)
					if err != nil {
						return nil
					}

					foundInstallPlan.Spec.Approved = true
					err = testHarness.Update(context.TODO(), foundInstallPlan)
					if err != nil {
						return nil
					}

					mdefList := &marketplacev1beta1.MeterDefinitionList{}
					err = testHarness.List(context.TODO(), mdefList)
					if err != nil {
						return nil
					}

					var mdefNames []string
					for _, mdef := range mdefList.Items {
						mdefNames = append(mdefNames, mdef.Name)
					}

					return mdefNames
				},timeout, interval).Should(ContainElements("memcached-meterdef-1","memcached-operator.v0.0.1-test-global-meterdef-pod-count-1", "memcached-operator.v0.0.1-test-global-meterdef-pod-count-2"),"apply meterdefs for 0.0.1")
			})
		})

		Context("update to memcached 0.0.2", func() {
			It("Should install the appropriate meterdefinitions if an operator is upgraded to a new version", func() {
				// install 0.0.1
				Eventually(func() []string {
					foundSub := &olmv1alpha1.Subscription{}
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-subscription", Namespace: "openshift-redhat-marketplace"}, foundSub)
					if err != nil {
						return nil
					}

					if foundSub.Status.InstallPlanRef == nil {
						return nil
					}

					installPlanName := foundSub.Status.InstallPlanRef.Name
					foundInstallPlan := &olmv1alpha1.InstallPlan{}
					err = testHarness.Get(context.TODO(), types.NamespacedName{Name: installPlanName, Namespace: "openshift-redhat-marketplace"}, foundInstallPlan)
					if err != nil {
						return nil
					}

					foundInstallPlan.Spec.Approved = true
					err = testHarness.Update(context.TODO(), foundInstallPlan)
					if err != nil {
						return nil
					}

					mdefList := &marketplacev1beta1.MeterDefinitionList{}
					err = testHarness.List(context.TODO(), mdefList)
					if err != nil {
						return nil
					}

					var mdefNames []string
					for _, mdef := range mdefList.Items {
						mdefNames = append(mdefNames, mdef.Name)
					}

					return mdefNames
				}, longTimeout, interval).Should(ContainElements("memcached-meterdef-1","memcached-operator.v0.0.1-test-global-meterdef-pod-count-1", "memcached-operator.v0.0.1-test-global-meterdef-pod-count-2"),"apply meterdefs for 0.0.1 during update")

				fmt.Println("upgrading to v0.0.2")
				Eventually(func() bool {
					memcachedCSV := &olmv1alpha1.ClusterServiceVersion{}
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-operator.v0.0.1", Namespace: "openshift-redhat-marketplace"}, memcachedCSV)
					if err != nil {
						return false
					}

					foundSub := &olmv1alpha1.Subscription{}
					err = testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-subscription", Namespace: "openshift-redhat-marketplace"}, foundSub)
					if err != nil {
						return false
					}

					if foundSub.Status.InstallPlanRef == nil {
						return false
					}

					foundSub.Spec.Channel = "beta"
					err = testHarness.Update(context.TODO(), foundSub)
					if err != nil {
						return false
					}

					return true
				}, timeout, interval).Should(BeTrue(), "update to beta channel (0.0.2)")

				foundInstallPlan := &olmv1alpha1.InstallPlan{}

				Eventually(func() string {
					updatedSub := &olmv1alpha1.Subscription{}
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-subscription", Namespace: "openshift-redhat-marketplace"}, updatedSub)
					if err != nil {
						return ""
					}

					installPlanName := updatedSub.Status.InstallPlanRef.Name

					err = testHarness.Get(context.TODO(), types.NamespacedName{Name: installPlanName, Namespace: "openshift-redhat-marketplace"}, foundInstallPlan)
					if err != nil {
						return ""
					}

					if foundInstallPlan.Spec.ClusterServiceVersionNames[0] == "" {
						return ""
					}

					return foundInstallPlan.Spec.ClusterServiceVersionNames[0]

				}, timeout, interval).Should(Equal("memcached-operator.v0.0.2"), "wait for install plan to populate with new csv info")

				Eventually(func() bool {
					foundInstallPlan.Spec.Approved = true
					err := testHarness.Update(context.TODO(), foundInstallPlan)
					return err == nil
				}, timeout, interval).Should(BeTrue(), "approve the install plan")

				Eventually(func() bool {
					memcachedCSV := &olmv1alpha1.ClusterServiceVersion{}
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-operator.v0.0.2", Namespace: "openshift-redhat-marketplace"}, memcachedCSV)
					return err == nil
				}, longTimeout, interval).Should(BeTrue(), "get updated csv")

				Eventually(func() []string {
					mdefList := &marketplacev1beta1.MeterDefinitionList{}
					err := testHarness.List(context.TODO(), mdefList)
					if err != nil {
						return nil
					}

					var mdefNames []string
					for _, mdef := range mdefList.Items {
						mdefNames = append(mdefNames, mdef.Name)
					}

					return mdefNames
				}, longTimeout, interval).Should(And(
					ContainElements("memcached-meterdef-2","memcached-operator.v0.0.2-test-global-meterdef-pod-count-1", "memcached-operator.v0.0.2-test-global-meterdef-pod-count-2"),
					Not(ContainElement("memcached-meterdef-1")),
				),"apply meterdefs for 0.0.2")
			})
		})
	})
})
