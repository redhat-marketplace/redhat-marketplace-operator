package controller_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	common "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/onsi/gomega/gstruct"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = FDescribe("MeterDefController reconcile", func() {
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
					CatalogSource:          "file-server-test-catalog",
					CatalogSourceNamespace: "openshift-redhat-marketplace",
				},
			}

			catalogSource = &olmv1alpha1.CatalogSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "file-server-test-catalog",
					Namespace: "openshift-redhat-marketplace",
				},
				Spec: olmv1alpha1.CatalogSourceSpec{
					SourceType: olmv1alpha1.SourceType(olmv1alpha1.SourceTypeGrpc),
					Image:      "quay.io/mxpaspa/my-index:1.0.0",
				},
			}

			Expect(testHarness.Create(context.TODO(), memcachedSub)).Should(SucceedOrAlreadyExist, "create the memcached subscription")
			Expect(testHarness.Create(context.TODO(), catalogSource)).Should(SucceedOrAlreadyExist, "create the test catalog")
		})

		AfterEach(func() {
			Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: "file-server-test-catalog", Namespace: "openshift-redhat-marketplace"}, catalogSource)).Should(Succeed())
			Expect(testHarness.Delete(context.TODO(), catalogSource)).Should(Succeed())

			memcachedSub := &olmv1alpha1.Subscription{}
			Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-subscription", Namespace: "openshift-redhat-marketplace"}, memcachedSub)).Should(Succeed())
			Expect(testHarness.Delete(context.TODO(), memcachedSub)).Should(Succeed())
		})

		Context("memcached 0.0.1", func() {
			AfterEach(func() {
				memcachedCSV := &olmv1alpha1.ClusterServiceVersion{}
				Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-operator.v0.0.1", Namespace: "openshift-redhat-marketplace"}, memcachedCSV)).Should(Succeed())
				Expect(testHarness.Delete(context.TODO(), memcachedCSV)).Should(Succeed())
				time.Sleep(time.Second * 10)
				Eventually(func() bool {
					installMapCm := &corev1.ConfigMap{}
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "rhm-meterdef-install-map", Namespace: "openshift-redhat-marketplace"}, installMapCm)
					if err != nil {
						return false
					}

					mdefStore := installMapCm.Data[utils.MeterDefinitionStoreKey]

					meterdefStore := &common.MeterdefinitionStore{}

					err = json.Unmarshal([]byte(mdefStore), meterdefStore)
					if err != nil {
						fmt.Println(err)
						return false
					}

					return len(meterdefStore.InstallMappings) == 0
				}, timeout, interval).Should(BeTrue(), "InstallMappings should not contain any items")

			})

			It("Should create a meterdef for 0.0.1 and update the install-map cm", func() {
				/* rhm-meterdef-install-map */
				Eventually(func() []common.InstallMapping {
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

					installMapCm := &corev1.ConfigMap{}
					err = testHarness.Get(context.TODO(), types.NamespacedName{Name: "rhm-meterdef-install-map", Namespace: "openshift-redhat-marketplace"}, installMapCm)
					if err != nil {
						return nil
					}

					mdefStore := installMapCm.Data[utils.MeterDefinitionStoreKey]

					meterdefStore := &common.MeterdefinitionStore{}

					err = json.Unmarshal([]byte(mdefStore), meterdefStore)
					if err != nil {
						fmt.Println(err)
						return nil
					}

					if len(meterdefStore.InstallMappings) > 0 {
						im := meterdefStore.InstallMappings
						utils.PrettyPrint(im)
						return im
					}

					return nil
				}, timeout, interval).Should(
					And(
						HaveLen(1),
						MatchAllElementsWithIndex(IndexIdentity,Elements{
							"0" : MatchAllFields(Fields{				
									"Namespace":  Equal("openshift-redhat-marketplace"),
									"CsvName":    Equal("memcached-operator"),
									"CsvVersion": Equal("0.0.1"),
									"InstalledMeterdefinitions": MatchAllElements(func(element interface{}) string {
										return string(element.(string))
									}, Elements{
										"memcached-meterdef-1": Not(BeZero()),
									}),
								}),
							}),
					),
				)
			})
		})

		Context("update to memcached 0.0.2", func() {
			AfterEach(func() {
				memcachedCSV := &olmv1alpha1.ClusterServiceVersion{}
				Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-operator.v0.0.2", Namespace: "openshift-redhat-marketplace"}, memcachedCSV)).Should(Succeed())
				Expect(testHarness.Delete(context.TODO(), memcachedCSV)).Should(Succeed())
				time.Sleep(time.Second * 10)
				Eventually(func() bool {
					installMapCm := &corev1.ConfigMap{}
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "rhm-meterdef-install-map", Namespace: "openshift-redhat-marketplace"}, installMapCm)
					if err != nil {
						return false
					}

					mdefStore := installMapCm.Data[utils.MeterDefinitionStoreKey]

					meterdefStore := &common.MeterdefinitionStore{}

					err = json.Unmarshal([]byte(mdefStore), meterdefStore)
					if err != nil {
						fmt.Println(err)
						return false
					}

					return len(meterdefStore.InstallMappings) == 0
				}, timeout, interval).Should(BeTrue(), "InstallMappings should not contain any items")
			})

			It("Should install the appropriate meterdefinitions if an operator is upgraded to a new version", func() {
				// install 0.0.1
				Eventually(func() []common.InstallMapping {
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

					installMapCm := &corev1.ConfigMap{}
					err = testHarness.Get(context.TODO(), types.NamespacedName{Name: "rhm-meterdef-install-map", Namespace: "openshift-redhat-marketplace"}, installMapCm)
					if err != nil {
						return nil
					}

					mdefStore := installMapCm.Data[utils.MeterDefinitionStoreKey]

					meterdefStore := &common.MeterdefinitionStore{}

					err = json.Unmarshal([]byte(mdefStore), meterdefStore)
					if err != nil {
						fmt.Println(err)
						return nil
					}

					if len(meterdefStore.InstallMappings) > 0 {
						im := meterdefStore.InstallMappings
						utils.PrettyPrint(im)
						return im
					}

					return nil
				}, timeout, interval).Should(
					And(
						HaveLen(1),
						MatchAllElementsWithIndex(IndexIdentity,Elements{
							"0" : MatchAllFields(Fields{				
									"Namespace":  Equal("openshift-redhat-marketplace"),
									"CsvName":    Equal("memcached-operator"),
									"CsvVersion": Equal("0.0.1"),
									"InstalledMeterdefinitions": MatchAllElements(func(element interface{}) string {
										return string(element.(string))
									}, Elements{
										"memcached-meterdef-1": Not(BeZero()),
									}),
								}),
							}),
					),
				)
			
				//upgrade to 0.0.2
				fmt.Println("installing v0.0.2")
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
				}, timeout, interval).Should(BeTrue())

				foundInstallPlan := &olmv1alpha1.InstallPlan{}

				Eventually(func() string {
					updatedSub := &olmv1alpha1.Subscription{}
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-subscription", Namespace: "openshift-redhat-marketplace"}, updatedSub)
					if err != nil {
						return ""
					}
					installPlanName := updatedSub.Status.InstallPlanRef.Name
					// foundInstallPlan := &olmv1alpha1.InstallPlan{}
					err = testHarness.Get(context.TODO(), types.NamespacedName{Name: installPlanName, Namespace: "openshift-redhat-marketplace"}, foundInstallPlan)
					if err != nil {
						return ""
					}

					if foundInstallPlan.Spec.ClusterServiceVersionNames[0] == "" {
						return ""
					}

					return foundInstallPlan.Spec.ClusterServiceVersionNames[0]

				}, timeout, interval).Should(Equal("memcached-operator.v0.0.2"))

				Eventually(func() bool {
					foundInstallPlan.Spec.Approved = true
					err := testHarness.Update(context.TODO(), foundInstallPlan)
					if err != nil {
						return false
					}

					return true
				}, timeout, interval).Should(BeTrue())

				Eventually(func() bool {
					memcachedCSV := &olmv1alpha1.ClusterServiceVersion{}
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "memcached-operator.v0.0.2", Namespace: "openshift-redhat-marketplace"}, memcachedCSV)
					if err != nil {
						return false
					}

					return true
				}, timeout, interval).Should(BeTrue())

				Eventually(func() []common.InstallMapping {
					installMapCm := &corev1.ConfigMap{}
					err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "rhm-meterdef-install-map", Namespace: "openshift-redhat-marketplace"}, installMapCm)
					if err != nil {
						return nil
					}

					mdefStore := installMapCm.Data[utils.MeterDefinitionStoreKey]

					meterdefStore := &common.MeterdefinitionStore{}

					err = json.Unmarshal([]byte(mdefStore), meterdefStore)
					if err != nil {
						return nil
					}

					if len(meterdefStore.InstallMappings) > 0 {
						im := meterdefStore.InstallMappings
						utils.PrettyPrint(im)
						return im
					}

					return nil
				}, timeout, interval).Should(
					And(
						HaveLen(1),
						MatchAllElementsWithIndex(IndexIdentity,Elements{
							"0" : MatchAllFields(Fields{				
									"Namespace":  Equal("openshift-redhat-marketplace"),
									"CsvName":    Equal("memcached-operator"),
									"CsvVersion": Equal("0.0.1"),
									"InstalledMeterdefinitions": MatchAllElements(func(element interface{}) string {
										return string(element.(string))
									}, Elements{
										"memcached-meterdef-1": Not(BeZero()),
									}),
								}),
							}),
					),
				)
			})
		})

	})

})
