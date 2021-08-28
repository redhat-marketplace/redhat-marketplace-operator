package marketplace

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/blang/semver"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"

	osappsv1 "github.com/openshift/api/apps/v1"

	"github.com/operator-framework/api/pkg/lib/version"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("DeploymentConfig Controller Test", func() {

	var (
		csvName      = "test-csv-1.v0.0.1"
		csvSplitName = "test-csv-1"
		namespace    = "default"
	)

	meterDef1Key := types.NamespacedName{
		Name:      "meterdef-1",
		Namespace: namespace,
	}

	meterDef2Key := types.NamespacedName{
		Name:      "meterdef-2",
		Namespace: namespace,
	}

	Status200 := 200
	listMeterDefsForCsvPath := "/" + catalog.ListForVersionEndpoint + "/" + csvSplitName + "/" + "0.0.1" + "/" + "default"
	systemMeterdefsPath := "/" + catalog.GetSystemMeterdefinitionTemplatesEndpoint


	Context("Install community meterdefs for a particular csv if no community meterdefs are on the cluster", func() {
		BeforeEach(func() {
			csvOnCluster := &olmv1alpha1.ClusterServiceVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      csvName,
					Namespace: namespace,
				},
				Spec: olmv1alpha1.ClusterServiceVersionSpec{
					InstallStrategy: olmv1alpha1.NamedInstallStrategy{
						StrategySpec: olmv1alpha1.StrategyDetailsDeployment{
							DeploymentSpecs: []olmv1alpha1.StrategyDeploymentSpec{},
						},
					},
					Version: version.OperatorVersion{
						Version: semver.Version{
							Major: 0,
							Minor: 0,
							Patch: 1,
						},
					},
				},
				Status: olmv1alpha1.ClusterServiceVersionStatus{},
			}

			Expect(k8sClient.Create(context.TODO(), csvOnCluster)).Should(Succeed(), "create test csv")

			returnedTemplatedMeterdefSlice := []marketplacev1beta1.MeterDefinition{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "MeterDefinition",
						APIVersion: "marketplace.redhat.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-template-meterdef",
						Namespace: "default",
						Annotations: map[string]string{
							"versionRange": "<=0.0.1",
						},
						Labels: map[string]string{
							"marketplace.redhat.com/installedOperatorNameTag":  "test-csv-1",
							"marketplace.redhat.com/isCommunityMeterdefintion": "1",
						},
					},
					Spec: marketplacev1beta1.MeterDefinitionSpec{
						Group: "apps.partner.metering.com",
						Kind:  "App",
						ResourceFilters: []marketplacev1beta1.ResourceFilter{
							{
								WorkloadType: marketplacev1beta1.WorkloadTypeService,
								OwnerCRD: &marketplacev1beta1.OwnerCRDFilter{
									GroupVersionKind: common.GroupVersionKind{
										APIVersion: "test_package_1.com/v2",
										Kind:       "test_package_1Cluster",
									},
								},
								Namespace: &marketplacev1beta1.NamespaceFilter{
									UseOperatorGroup: true,
								},
							},
						},
						Meters: []marketplacev1beta1.MeterWorkload{
							{
								Aggregation: "sum",
								GroupBy:     []string{"namespace"},
								Period: &metav1.Duration{
									Duration: time.Duration(time.Hour * 1),
								},
								Query:        "kube_service_labels{}",
								Metric:       "test_package_1_cluster_count",
								WorkloadType: marketplacev1beta1.WorkloadTypeService,
								Without:      []string{"label_test_package_1_cluster", "label_app", "label_operator_test_package_1_com_version"},
							},
						},
					},
				},
			}

			templatedMeterdefBody, err := json.Marshal(returnedTemplatedMeterdefSlice)
			if err != nil {
				log.Fatal(err)
			}

			dcControllerMockServer.RouteToHandler(
				"POST", systemMeterdefsPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", systemMeterdefsPath),
					ghttp.RespondWithPtr(&Status200, &templatedMeterdefBody),
				))

			indexLabelsPath := "/" + catalog.GetMeterdefinitionIndexLabelEndpoint + "/" + csvSplitName

			indexLabelsBody := []byte(`{
					"marketplace.redhat.com/installedOperatorNameTag": "test-csv-1",
					"marketplace.redhat.com/isCommunityMeterdefintion": "1"
				}`)

			dcControllerMockServer.RouteToHandler(
				"GET", indexLabelsPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", indexLabelsPath),
					ghttp.RespondWithPtr(&Status200, &indexLabelsBody),
				))

			returnedMeterdefGoSlice := []marketplacev1beta1.MeterDefinition{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "MeterDefinition",
						APIVersion: "marketplace.redhat.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      meterDef1Key.Name,
						Namespace: "default",
						Annotations: map[string]string{
							"versionRange": "<=0.0.1",
						},
						Labels: map[string]string{
							"marketplace.redhat.com/installedOperatorNameTag":  "test-csv-1",
							"marketplace.redhat.com/isCommunityMeterdefintion": "1",
						},
					},
					Spec: marketplacev1beta1.MeterDefinitionSpec{
						Group: "apps.partner.metering.com",
						Kind:  "App",
						ResourceFilters: []marketplacev1beta1.ResourceFilter{
							{
								WorkloadType: marketplacev1beta1.WorkloadTypeService,
								OwnerCRD: &marketplacev1beta1.OwnerCRDFilter{
									GroupVersionKind: common.GroupVersionKind{
										APIVersion: "test_package_1.com/v2",
										Kind:       "test_package_1Cluster",
									},
								},
								Namespace: &marketplacev1beta1.NamespaceFilter{
									UseOperatorGroup: true,
								},
							},
						},
						Meters: []marketplacev1beta1.MeterWorkload{
							{
								Aggregation: "sum",
								GroupBy:     []string{"namespace"},
								Period: &metav1.Duration{
									Duration: time.Duration(time.Hour * 1),
								},
								Query:        "kube_service_labels{}",
								Metric:       "test_package_1_cluster_count",
								WorkloadType: marketplacev1beta1.WorkloadTypeService,
								Without:      []string{"label_test_package_1_cluster", "label_app", "label_operator_test_package_1_com_version"},
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "MeterDefinition",
						APIVersion: "marketplace.redhat.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      meterDef2Key.Name,
						Namespace: "default",
						Annotations: map[string]string{
							"versionRange": "<=0.0.1",
						},
						Labels: map[string]string{
							"marketplace.redhat.com/installedOperatorNameTag":  "test-csv-1",
							"marketplace.redhat.com/isCommunityMeterdefintion": "1",
						},
					},
					Spec: marketplacev1beta1.MeterDefinitionSpec{
						Group: "apps.partner.metering.com",
						Kind:  "App",
						ResourceFilters: []marketplacev1beta1.ResourceFilter{
							{
								WorkloadType: marketplacev1beta1.WorkloadTypeService,
								OwnerCRD: &marketplacev1beta1.OwnerCRDFilter{
									GroupVersionKind: common.GroupVersionKind{
										APIVersion: "test_package_1.com/v2",
										Kind:       "test_package_1Cluster",
									},
								},
								Namespace: &marketplacev1beta1.NamespaceFilter{
									UseOperatorGroup: true,
								},
							},
						},
						Meters: []marketplacev1beta1.MeterWorkload{
							{
								Aggregation: "sum",
								GroupBy:     []string{"namespace"},
								Period: &metav1.Duration{
									Duration: time.Duration(time.Hour * 1),
								},
								Query:        "kube_service_labels{}",
								Metric:       "test_package_1_cluster_count",
								WorkloadType: marketplacev1beta1.WorkloadTypeService,
								Without:      []string{"label_test_package_1_cluster", "label_app", "label_operator_test_package_1_com_version"},
							},
						},
					},
				},
			}

			meterdefForCsvBody, err := json.Marshal(returnedMeterdefGoSlice)
			if err != nil {
				log.Fatal(err)
			}

			dcControllerMockServer.RouteToHandler(
				"GET", listMeterDefsForCsvPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", listMeterDefsForCsvPath),
					ghttp.RespondWithPtr(&Status200, &meterdefForCsvBody),
				))
		})

		It("Should find meterdef for test-csv-1", func() {
			Eventually(func() string {
				found := &marketplacev1beta1.MeterDefinition{}
				k8sClient.Get(context.TODO(), meterDef1Key, found)
				return found.Name
			}, timeout, interval).Should(Equal(meterDef1Key.Name))
		})
	})

	Context("Update a community meterdefinition on the cluster", func() {
		BeforeEach(func() {
			_dc := &osappsv1.DeploymentConfig{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: "default"}, _dc)).Should(Succeed(), "get deployment config")

			_dc.Status.LatestVersion = _dc.Status.LatestVersion + 1

			Expect(k8sClient.Update(context.TODO(), _dc)).Should(Succeed(), "update deploymentconfig")

			updateMeterdefGoSlice := []marketplacev1beta1.MeterDefinition{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "MeterDefinition",
						APIVersion: "marketplace.redhat.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      meterDef1Key.Name,
						Namespace: "default",
						Annotations: map[string]string{
							"versionRange": "<=0.0.1",
						},
						Labels: map[string]string{
							"marketplace.redhat.com/installedOperatorNameTag":  "test-csv-1",
							"marketplace.redhat.com/isCommunityMeterdefintion": "1",
						},
					},
					Spec: marketplacev1beta1.MeterDefinitionSpec{
						Group: "apps.partner.metering.com",
						Kind:  "App",
						ResourceFilters: []marketplacev1beta1.ResourceFilter{
							{
								WorkloadType: marketplacev1beta1.WorkloadTypeService,
								OwnerCRD: &marketplacev1beta1.OwnerCRDFilter{
									GroupVersionKind: common.GroupVersionKind{
										APIVersion: "test_package_1.com/v2",
										Kind:       "test_package_1Cluster",
									},
								},
								Namespace: &marketplacev1beta1.NamespaceFilter{
									UseOperatorGroup: true,
								},
							},
						},
						Meters: []marketplacev1beta1.MeterWorkload{
							{
								//UPDATE
								Name:        "updated",
								Aggregation: "sum",
								GroupBy:     []string{"namespace"},
								Period: &metav1.Duration{
									Duration: time.Duration(time.Hour * 1),
								},
								Query:        "kube_service_labels{}",
								Metric:       "test_package_1_cluster_count",
								WorkloadType: marketplacev1beta1.WorkloadTypeService,
								Without:      []string{"label_test_package_1_cluster", "label_app", "label_operator_test_package_1_com_version"},
							},
						},
					},
				},
			}

			meterdefForCsvBody, err := json.Marshal(updateMeterdefGoSlice)
			if err != nil {
				log.Fatal(err)
			}

			dcControllerMockServer.RouteToHandler(
				"GET", listMeterDefsForCsvPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", listMeterDefsForCsvPath),
					ghttp.RespondWithPtr(&Status200, &meterdefForCsvBody),
				))
		})

		It("meterdefintion on cluster should be updated", func() {

			Eventually(func() string {
				found := &marketplacev1beta1.MeterDefinition{}
				k8sClient.Get(context.TODO(), meterDef1Key, found)

				if found != nil {
					return found.Spec.Meters[0].Name
				}
				return ""
			}, timeout, interval).Should(Equal("updated"))
		})
	})

	Context("Remove a community meterdefinition if it is removed from the catalog", func() {
		BeforeEach(func() {
			_dc := &osappsv1.DeploymentConfig{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: "default"}, _dc)).Should(Succeed(), "get deployment config")

			_dc.Status.LatestVersion = _dc.Status.LatestVersion + 1

			Expect(k8sClient.Update(context.TODO(), _dc)).Should(Succeed(), "update deploymentconfig")

			updateMeterdefGoSlice := []marketplacev1beta1.MeterDefinition{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "MeterDefinition",
						APIVersion: "marketplace.redhat.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      meterDef1Key.Name,
						Namespace: "default",
						Annotations: map[string]string{
							"versionRange": "<=0.0.1",
						},
						Labels: map[string]string{
							"marketplace.redhat.com/installedOperatorNameTag":  "test-csv-1",
							"marketplace.redhat.com/isCommunityMeterdefintion": "1",
						},
					},
					Spec: marketplacev1beta1.MeterDefinitionSpec{
						Group: "apps.partner.metering.com",
						Kind:  "App",
						ResourceFilters: []marketplacev1beta1.ResourceFilter{
							{
								WorkloadType: marketplacev1beta1.WorkloadTypeService,
								OwnerCRD: &marketplacev1beta1.OwnerCRDFilter{
									GroupVersionKind: common.GroupVersionKind{
										APIVersion: "test_package_1.com/v2",
										Kind:       "test_package_1Cluster",
									},
								},
								Namespace: &marketplacev1beta1.NamespaceFilter{
									UseOperatorGroup: true,
								},
							},
						},
						Meters: []marketplacev1beta1.MeterWorkload{
							{
								Aggregation: "sum",
								GroupBy:     []string{"namespace"},
								Period: &metav1.Duration{
									Duration: time.Duration(time.Hour * 1),
								},
								Query:        "kube_service_labels{}",
								Metric:       "test_package_1_cluster_count",
								WorkloadType: marketplacev1beta1.WorkloadTypeService,
								Without:      []string{"label_test_package_1_cluster", "label_app", "label_operator_test_package_1_com_version"},
							},
						},
					},
				},
			}

			meterdefForCsvBody, err := json.Marshal(updateMeterdefGoSlice)
			if err != nil {
				log.Fatal(err)
			}

			dcControllerMockServer.RouteToHandler(
				"GET", listMeterDefsForCsvPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", listMeterDefsForCsvPath),
					ghttp.RespondWithPtr(&Status200, &meterdefForCsvBody),
				))
		})

		It("meterdef-2 should be deleted", func() {
			Eventually(func() bool {
				mdefList := &marketplacev1beta1.MeterDefinitionList{}
				k8sClient.List(context.TODO(), mdefList)

				for _,mdef := range mdefList.Items {
					if mdef.Name == meterDef2Key.Name {
						return false
					}
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Delete all community meterdefs for a csv if that csv's dir is deleted in the catalog", func() {
		BeforeEach(func() {
			_dc := &osappsv1.DeploymentConfig{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: "default"}, _dc)).Should(Succeed(), "get deployment config")

			_dc.Status.LatestVersion = _dc.Status.LatestVersion + 1

			Expect(k8sClient.Update(context.TODO(), _dc)).Should(Succeed(), "update deploymentconfig")

			notFoundBody := []byte(`no meterdefs found`)

			dcControllerMockServer.RouteToHandler(
				"GET", listMeterDefsForCsvPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", listMeterDefsForCsvPath),
					ghttp.RespondWith(http.StatusNoContent, notFoundBody),
				))
		})

		It("all community meterdefinitions should be deleted", func() {
			Eventually(func() bool {
				mdefList := &marketplacev1beta1.MeterDefinitionList{}
				k8sClient.List(context.TODO(), mdefList)

				return len(mdefList.Items) == 0
			}, timeout, interval).Should(BeTrue())
		})
	})
})
