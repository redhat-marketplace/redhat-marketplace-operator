package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/blang/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/onsi/gomega/ghttp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/gotidy/ptr"

	osappsv1 "github.com/openshift/api/apps/v1"
	osimagev1 "github.com/openshift/api/image/v1"
	"github.com/operator-framework/api/pkg/lib/version"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = FDescribe("DeploymentConfig Controller Test", func() {

	var (
		csvName      = "test-csv-1.v0.0.1"
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

	meterDef1 := marketplacev1beta1.MeterDefinition{
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
	}

	meterDef2 := marketplacev1beta1.MeterDefinition{
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
	}

	csvOnCluster := olmv1alpha1.ClusterServiceVersion{
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

	Status200 := 200
	// listMeterDefsForCsvPath := "/" + catalog.ListForVersionEndpoint + "/" + csvSplitName + "/" + "0.0.1" + "/" + "default"
	systemMeterdefsPath := "/" + catalog.GetSystemMeterdefinitionTemplatesEndpoint

	BeforeEach(func() {
		subSectionMeterBase := &marketplacev1alpha1.MeterBase{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.METERBASE_NAME,
				Namespace: namespace,
			},
			Spec: marketplacev1alpha1.MeterBaseSpec{
				Enabled: false,
				Prometheus: &marketplacev1alpha1.PrometheusSpec{
					Storage: marketplacev1alpha1.StorageSpec{
						Size: resource.MustParse("30Gi"),
					},
					Replicas: ptr.Int32(2),
				},
				MeterdefinitionCatalogServer: &marketplacev1alpha1.MeterdefinitionCatalogServerSpec{
					MeterdefinitionCatalogServerEnabled: true,
					LicenceUsageMeteringEnabled:         true,
				},
			},
		}

		dc := &osappsv1.DeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.DEPLOYMENT_CONFIG_NAME,
				Namespace: "default",
			},
			Spec: osappsv1.DeploymentConfigSpec{
				Triggers: osappsv1.DeploymentTriggerPolicies{
					{
						Type: osappsv1.DeploymentTriggerOnConfigChange,
						ImageChangeParams: &osappsv1.DeploymentTriggerImageChangeParams{
							Automatic:      true,
							ContainerNames: []string{"rhm-meterdefinition-file-server"},
							From: corev1.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "rhm-meterdefinition-file-server:v1",
							},
						},
					},
				},
			},
			Status: osappsv1.DeploymentConfigStatus{
				LatestVersion: 1,
				Conditions: []osappsv1.DeploymentCondition{
					{
						Type:               osappsv1.DeploymentConditionType(osappsv1.DeploymentAvailable),
						Reason:             "NewReplicationControllerAvailable",
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
						LastUpdateTime:     metav1.Now(),
					},
				},
			},
		}

		is := &osimagev1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.DEPLOYMENT_CONFIG_NAME,
				Namespace: "default",
			},
			Spec: osimagev1.ImageStreamSpec{
				LookupPolicy: osimagev1.ImageLookupPolicy{
					Local: false,
				},
				Tags: []osimagev1.TagReference{
					{
						Annotations: map[string]string{
							"openshift.io/imported-from": "quay.io/mxpaspa/rhm-meterdefinition-file-server:return-204-1.0.0",
						},
						From: &corev1.ObjectReference{
							Name: "quay.io/mxpaspa/rhm-meterdefinition-file-server:return-204-1.0.0",
							Kind: "DockerImage",
						},
						ImportPolicy: osimagev1.TagImportPolicy{
							Insecure:  true,
							Scheduled: true,
						},
						Name: "v1",
						ReferencePolicy: osimagev1.TagReferencePolicy{
							Type: osimagev1.SourceTagReferencePolicy,
						},
						Generation: ptr.Int64(1),
					},
				},
			},
		}

		Expect(k8sClient.Create(context.TODO(), dc)).Should(SucceedOrAlreadyExist, "create test deploymentconfig")
		Expect(k8sClient.Create(context.TODO(), is)).Should(SucceedOrAlreadyExist, "create test image stream")
		Expect(k8sClient.Create(context.TODO(), subSectionMeterBase)).Should(SucceedOrAlreadyExist, "create sub-section meterbase")

		// indexLabelsPath := "/" + catalog.GetMeterdefinitionIndexLabelEndpoint + "/" + csvSplitName

		// indexLabelsBody := []byte(`{
		// 		"marketplace.redhat.com/installedOperatorNameTag": "test-csv-1",
		// 		"marketplace.redhat.com/isCommunityMeterdefintion": "1"
		// 	}`)

		// dcControllerMockServer.RouteToHandler(
		// 	"GET", indexLabelsPath, ghttp.CombineHandlers(
		// 		ghttp.VerifyRequest("GET", indexLabelsPath),
		// 		ghttp.RespondWithPtr(&Status200, &indexLabelsBody),
		// 	))

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

	})

	AfterEach(func() {
		Eventually(func() []marketplacev1beta1.MeterDefinition {
			mdefList := &marketplacev1beta1.MeterDefinitionList{}
			k8sClient.List(context.TODO(), mdefList)

			for _, mdef := range mdefList.Items {
				k8sClient.Delete(context.TODO(), &mdef)
			}

			mdefList = &marketplacev1beta1.MeterDefinitionList{}
			k8sClient.List(context.TODO(), mdefList)
			return mdefList.Items
		}, timeout, interval).Should(HaveLen(0))

		// _csv := &olmv1alpha1.ClusterServiceVersion{}
		// Expect(k8sClient.Get(context.TODO(),types.NamespacedName{Name:csvName,Namespace: "default"}, _csv)).Should(Succeed())
		// Expect(k8sClient.Delete(context.TODO(), _csv)).Should(Succeed())
		Eventually(func() []olmv1alpha1.ClusterServiceVersion {
			csvList := &olmv1alpha1.ClusterServiceVersionList{}
			k8sClient.List(context.TODO(), csvList)

			for _, csv := range csvList.Items {
				k8sClient.Delete(context.TODO(), &csv)
			}

			csvList = &olmv1alpha1.ClusterServiceVersionList{}
			k8sClient.List(context.TODO(), csvList)
			return csvList.Items
		}, timeout, interval).Should(HaveLen(0))
	})

	Describe("Run sync funcs", func() {
		Context("create a community meterdef if not found", func() {
			BeforeEach(func() {
				_csvOnCluster := csvOnCluster.DeepCopy()
				_csvOnCluster.Name = "test-create.v0.0.1"
				_csvSplitName := "test-create"
				Expect(k8sClient.Create(context.TODO(), _csvOnCluster)).Should(SucceedOrAlreadyExist, "create csv for create if not found test")

				_dc := &osappsv1.DeploymentConfig{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: namespace}, _dc)).Should(Succeed(), "get deployment config")

				_dc.Status.LatestVersion = _dc.Status.LatestVersion + 1

				Expect(k8sClient.Update(context.TODO(), _dc)).Should(Succeed(), "update deploymentconfig")

				indexLabelsPath := "/" + catalog.GetMeterdefinitionIndexLabelEndpoint + "/" + _csvSplitName

				indexLabelsBody := []byte(fmt.Sprintf(`{
						"marketplace.redhat.com/installedOperatorNameTag":"%v",
						"marketplace.redhat.com/isCommunityMeterdefintion": "1"
					}`,_csvSplitName))

				dcControllerMockServer.RouteToHandler(
					"GET", indexLabelsPath, ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", indexLabelsPath),
						ghttp.RespondWithPtr(&Status200, &indexLabelsBody),
					))

				createdMeterdef := meterDef1.DeepCopy()
				createdMeterdef.Labels["marketplace.redhat.com/installedOperatorNameTag"] = _csvSplitName
				returnedMeterdefGoSlice := []marketplacev1beta1.MeterDefinition{*createdMeterdef}

				meterdefForCsvBody, err := json.Marshal(returnedMeterdefGoSlice)
				if err != nil {
					log.Fatal(err)
				}

				listMeterDefsForCsvPath := "/" + catalog.ListForVersionEndpoint + "/" + _csvSplitName + "/" + "0.0.1" + "/" + namespace

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
				_csvOnCluster := csvOnCluster.DeepCopy()
				_csvOnCluster.Name = "test-update.v0.0.1"
				_csvSplitName := "test-update"
				Expect(k8sClient.Create(context.TODO(), _csvOnCluster)).Should(SucceedOrAlreadyExist, "create csv for update test")

				indexLabelsPath := "/" + catalog.GetMeterdefinitionIndexLabelEndpoint + "/" + _csvSplitName

				indexLabelsBody := []byte(fmt.Sprintf(`{
						"marketplace.redhat.com/installedOperatorNameTag": "%v",
						"marketplace.redhat.com/isCommunityMeterdefintion": "1"
					}`,_csvSplitName))

				dcControllerMockServer.RouteToHandler(
					"GET", indexLabelsPath, ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", indexLabelsPath),
						ghttp.RespondWithPtr(&Status200, &indexLabelsBody),
					))

				_dc := &osappsv1.DeploymentConfig{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: namespace}, _dc)).Should(Succeed(), "get deployment config")

				_dc.Status.LatestVersion = _dc.Status.LatestVersion + 1

				Expect(k8sClient.Update(context.TODO(), _dc)).Should(Succeed(), "update deploymentconfig")

				existingMeterdef := meterDef1.DeepCopy()
				existingMeterdef.Labels["marketplace.redhat.com/installedOperatorNameTag"] = _csvSplitName
				Expect(k8sClient.Create(context.TODO(), existingMeterdef)).Should(SucceedOrAlreadyExist, "create existing meterdef")

				updatedMeterDef := meterDef1.DeepCopy()
				updatedMeterDef.Labels["marketplace.redhat.com/installedOperatorNameTag"] = _csvSplitName
				updatedMeterDef.Spec.Meters[0].Name = "updated"
				updateMeterdefGoSlice := []marketplacev1beta1.MeterDefinition{*updatedMeterDef}

				meterdefForCsvBody, err := json.Marshal(updateMeterdefGoSlice)
				if err != nil {
					log.Fatal(err)
				}

				listMeterDefsForCsvPath := "/" + catalog.ListForVersionEndpoint + "/" + _csvSplitName + "/" + "0.0.1" + "/" + namespace

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
				_csvOnCluster := csvOnCluster.DeepCopy()
				_csvOnCluster.Name = "test-delete-meterdef.v0.0.1"
				_csvSplitName := "test-delete-meterdef"
				Expect(k8sClient.Create(context.TODO(), _csvOnCluster)).Should(SucceedOrAlreadyExist, "create csv for delete test")

				indexLabelsPath := "/" + catalog.GetMeterdefinitionIndexLabelEndpoint + "/" + _csvSplitName
				
				indexLabelsBody := []byte(fmt.Sprintf(`{
						"marketplace.redhat.com/installedOperatorNameTag":"%v",
						"marketplace.redhat.com/isCommunityMeterdefintion": "1"
					}`,_csvSplitName))

				dcControllerMockServer.RouteToHandler(
					"GET", indexLabelsPath, ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", indexLabelsPath),
						ghttp.RespondWithPtr(&Status200, &indexLabelsBody),
					))

				_dc := &osappsv1.DeploymentConfig{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: "default"}, _dc)).Should(Succeed(), "get deployment config")

				_dc.Status.LatestVersion = _dc.Status.LatestVersion + 1

				Expect(k8sClient.Update(context.TODO(), _dc)).Should(Succeed(), "update deploymentconfig")

				_meterDef1 := meterDef1.DeepCopy()
				_meterDef1.Labels["marketplace.redhat.com/installedOperatorNameTag"] = _csvSplitName

				_meterDef2 := meterDef2.DeepCopy()
				_meterDef2.Labels["marketplace.redhat.com/installedOperatorNameTag"] = _csvSplitName
				
				existingMeterdefSlice := []marketplacev1beta1.MeterDefinition{*_meterDef1,*_meterDef2}

				for _, existingMeterdef := range existingMeterdefSlice {
					Expect(k8sClient.Create(context.TODO(), &existingMeterdef)).Should(SucceedOrAlreadyExist, "create existing meterdefs")
				}

				latestMeterdefsFromCatalog := []marketplacev1beta1.MeterDefinition{*_meterDef1}

				meterdefForCsvBody, err := json.Marshal(latestMeterdefsFromCatalog)
				if err != nil {
					log.Fatal(err)
				}

				listMeterDefsForCsvPath := "/" + catalog.ListForVersionEndpoint + "/" + _csvSplitName + "/" + "0.0.1" + "/" + namespace

				dcControllerMockServer.RouteToHandler(
					"GET", listMeterDefsForCsvPath, ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", listMeterDefsForCsvPath),
						ghttp.RespondWithPtr(&Status200, &meterdefForCsvBody),
					))
			})

			It("meterdef-2 should be deleted", func() {
				Eventually(func() []string {
					mdefList := &marketplacev1beta1.MeterDefinitionList{}
					k8sClient.List(context.TODO(), mdefList)

					var mdefNames []string
					for _, mdef := range mdefList.Items {
						mdefNames = append(mdefNames, mdef.Name)
					}

					return mdefNames
				}, timeout, interval).Should(And(
					HaveLen(2),
					MatchAllElementsWithIndex(IndexIdentity, Elements{
						"0": Equal("meterdef-1"),
						"1": Equal("test-template-meterdef"),
					}),
				))
			})
		})

		Context("Delete all community meterdefs for a csv if that csv's dir is deleted in the catalog", func() {
			BeforeEach(func() {
				_csvOnCluster := csvOnCluster.DeepCopy()
				_csvOnCluster.Name = "test-delete-all.v0.0.1"
				_csvSplitName := "test-delete-all"
				Expect(k8sClient.Create(context.TODO(), _csvOnCluster)).Should(SucceedOrAlreadyExist, "create csv for delete all meterdefs test")

				indexLabelsPath := "/" + catalog.GetMeterdefinitionIndexLabelEndpoint + "/" + _csvSplitName

				indexLabelsBody := []byte(fmt.Sprintf(`{
						"marketplace.redhat.com/installedOperatorNameTag": "%v",
						"marketplace.redhat.com/isCommunityMeterdefintion": "1"
					}`,_csvSplitName))

				dcControllerMockServer.RouteToHandler(
					"GET", indexLabelsPath, ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", indexLabelsPath),
						ghttp.RespondWithPtr(&Status200, &indexLabelsBody),
					))

				_dc := &osappsv1.DeploymentConfig{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: "default"}, _dc)).Should(Succeed(), "get deployment config")

				_dc.Status.LatestVersion = _dc.Status.LatestVersion + 1

				Expect(k8sClient.Update(context.TODO(), _dc)).Should(Succeed(), "update deploymentconfig")
				
				_meterDef1 := meterDef1.DeepCopy()
				_meterDef1.Labels["marketplace.redhat.com/installedOperatorNameTag"] = _csvSplitName

				_meterDef2 := meterDef2.DeepCopy()
				_meterDef2.Labels["marketplace.redhat.com/installedOperatorNameTag"] = _csvSplitName
				existingMeterdefSlice := []marketplacev1beta1.MeterDefinition{*_meterDef1,*_meterDef2}

				for _, existingMeterdef := range existingMeterdefSlice {
					Expect(k8sClient.Create(context.TODO(), &existingMeterdef)).Should(SucceedOrAlreadyExist, "create existing meterdefs")
				}

				notFoundBody := []byte(`no meterdefs found`)

				listMeterDefsForCsvPath := "/" + catalog.ListForVersionEndpoint + "/" + _csvSplitName + "/" + "0.0.1" + "/" + "default"

				dcControllerMockServer.RouteToHandler(
					"GET", listMeterDefsForCsvPath, ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", listMeterDefsForCsvPath),
						ghttp.RespondWith(http.StatusNoContent, notFoundBody),
					))
			})

			It("all community meterdefinitions should be deleted", func() {
				Eventually(func() []marketplacev1beta1.MeterDefinition {
					mdefList := &marketplacev1beta1.MeterDefinitionList{}
					k8sClient.List(context.TODO(), mdefList)

					return mdefList.Items
				}, timeout, interval).Should(HaveLen(0))
			})
		})
	})
})
