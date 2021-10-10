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
	"encoding/json"
	"log"
	"net"
	"time"

	"github.com/blang/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	// . "github.com/onsi/gomega/gstruct"

	"github.com/onsi/gomega/ghttp"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/gotidy/ptr"

	"github.com/operator-framework/api/pkg/lib/version"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = FDescribe("MeterDefinitionInstall Controller Test", func() {

	var (
		namespace = "default"

		/* rhm csv */
		csvName           = "test-csv-1.v0.0.1"
		subName           = "test-csv-1-sub"
		packageName       = "test-csv-1-rhmp"
		catalogSourceName = "redhat-marketplace"

		/* non-rhm csv */
		nonRhmCsvName           = "non-rhm-csv.v0.0.1"
		nonRhmSubNmae           = "non-rhm-sub"
		nonRhmPackageName       = "non-rhm-package"
		nonRhmCatalogSourceName = "non-rhm-catalog-source"

		/* system meterdefs */
		systemMeterDef1Name = csvName + "-" + "pod-count"
		systemMeterDef2Name = csvName + "-" + "cpu-usage"

		/* paths */
		communityMeterDefsPath = "/" + catalog.GetCommunityMeterdefinitionsEndpoint
		systemMeterdefsPath    = "/" + catalog.GetSystemMeterdefinitionTemplatesEndpoint + "/" + packageName + "/" + catalogSourceName
		dcControllerMockServer *ghttp.Server

		Status200 = 200
	)

	// idFn := func(element interface{}) string {
	// 	return fmt.Sprintf("%v", element)
	// }

	meterDef1Key := types.NamespacedName{
		Name:      "meterdef-1",
		Namespace: namespace,
	}

	meterDef2Key := types.NamespacedName{
		Name:      "meterdef-2",
		Namespace: namespace,
	}

	systemMeterDef1Key := types.NamespacedName{
		Name:      systemMeterDef1Name,
		Namespace: namespace,
	}

	systemMeterDef2Key := types.NamespacedName{
		Name:      systemMeterDef2Name,
		Namespace: namespace,
	}

	rhmCsvKey := types.NamespacedName{
		Name:      csvName,
		Namespace: namespace,
	}

	meterBaseKey := types.NamespacedName{
		Name:      utils.METERBASE_NAME,
		Namespace: namespace,
	}

	systemMeterDef1 := marketplacev1beta1.MeterDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MeterDefinition",
			APIVersion: "marketplace.redhat.com/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      systemMeterDef1Name,
			Namespace: "default",
			Annotations: map[string]string{
				"versionRange":        "<=0.0.1",
				"subscription.source": catalogSourceName,
				"subscription.name":   packageName,
			},
		},
		Spec: marketplacev1beta1.MeterDefinitionSpec{
			Group: "apps.partner.metering.com",
			Kind:  "App",
			ResourceFilters: []marketplacev1beta1.ResourceFilter{
				{
					WorkloadType: common.WorkloadTypeService,
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
					WorkloadType: common.WorkloadTypeService,
					Without:      []string{"label_test_package_1_cluster", "label_app", "label_operator_test_package_1_com_version"},
				},
			},
		},
	}

	meterDef1 := marketplacev1beta1.MeterDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MeterDefinition",
			APIVersion: "marketplace.redhat.com/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      meterDef1Key.Name,
			Namespace: namespace,
			Annotations: map[string]string{
				"versionRange":        "<=0.0.1",
				"subscription.source": catalogSourceName,
				"subscription.name":   packageName,
			},
		},
		Spec: marketplacev1beta1.MeterDefinitionSpec{
			Group: "apps.partner.metering.com",
			Kind:  "App",
			ResourceFilters: []marketplacev1beta1.ResourceFilter{
				{
					WorkloadType: common.WorkloadTypeService,
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
					WorkloadType: common.WorkloadTypeService,
					Without:      []string{"label_test_package_1_cluster", "label_app", "label_operator_test_package_1_com_version"},
				},
			},
		},
	}

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
			MeterdefinitionCatalogServerConfig: &common.MeterDefinitionCatalogServerConfig{
				SyncCommunityMeterDefinitions:      true,
				SyncSystemMeterDefinitions:         true,
				DeployMeterDefinitionCatalogServer: true,
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

	subs := []olmv1alpha1.Subscription{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      subName,
				Namespace: namespace,
				Labels: map[string]string{
					operatorTag: "true",
				},
			},
			Spec: &olmv1alpha1.SubscriptionSpec{
				CatalogSource:          catalogSourceName,
				CatalogSourceNamespace: namespace,
				Package:                packageName,
			},
			Status: olmv1alpha1.SubscriptionStatus{
				InstalledCSV: csvName,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nonRhmSubNmae,
				Namespace: namespace,
			},
			Spec: &olmv1alpha1.SubscriptionSpec{
				CatalogSource:          nonRhmCatalogSourceName,
				CatalogSourceNamespace: namespace,
				Package:                nonRhmPackageName,
			},
			Status: olmv1alpha1.SubscriptionStatus{
				InstalledCSV: nonRhmCsvName,
			},
		},
	}

	BeforeEach(func() {
		customListener, err := net.Listen("tcp", listenerAddress)
		Expect(err).ToNot(HaveOccurred())

		dcControllerMockServer = ghttp.NewUnstartedServer()
		dcControllerMockServer.HTTPTestServer.Listener.Close()
		dcControllerMockServer.HTTPTestServer.Listener = customListener
		dcControllerMockServer.SetAllowUnhandledRequests(true)
		dcControllerMockServer.Start()
	})

	AfterEach(func() {
		dcControllerMockServer.Close()

		meterDef1 := &marketplacev1beta1.MeterDefinition{}
		k8sClient.Get(context.TODO(), meterDef1Key, meterDef1)
		k8sClient.Delete(context.TODO(), meterDef1)

		meterDef2 := &marketplacev1beta1.MeterDefinition{}
		k8sClient.Get(context.TODO(), meterDef2Key, meterDef2)
		k8sClient.Delete(context.TODO(), meterDef2)

		systemMeterDef1 := &marketplacev1beta1.MeterDefinition{}
		k8sClient.Get(context.TODO(), systemMeterDef1Key, systemMeterDef1)
		k8sClient.Delete(context.TODO(), systemMeterDef1)

		systemMeterDef2 := &marketplacev1beta1.MeterDefinition{}
		k8sClient.Get(context.TODO(), systemMeterDef2Key, systemMeterDef2)
		k8sClient.Delete(context.TODO(), systemMeterDef2)

		csv := &olmv1alpha1.ClusterServiceVersion{}
		k8sClient.Get(context.TODO(), rhmCsvKey, csv)
		k8sClient.Delete(context.TODO(), csv)

		meterBase := &marketplacev1alpha1.MeterBase{}
		Expect(k8sClient.Get(context.TODO(), meterBaseKey, meterBase)).Should(Succeed(), "get meterbase")
		k8sClient.Delete(context.TODO(), meterBase)
	})

	FContext("Create", func() {
		BeforeEach(func() {
			listSubs = func(k8sclient client.Client) ([]olmv1alpha1.Subscription, error) {
				return subs, nil
			}

			Expect(k8sClient.Create(context.TODO(), csvOnCluster.DeepCopy())).Should(Succeed(), "create csv on cluster")
			Expect(k8sClient.Create(context.TODO(), subSectionMeterBase.DeepCopy())).Should(Succeed(), "create sub-section meterbase")

			returnedCommunityMeterdefGoSlice := []marketplacev1beta1.MeterDefinition{*meterDef1.DeepCopy()}
			communityMeterDefsBody, err := json.Marshal(returnedCommunityMeterdefGoSlice)
			if err != nil {
				log.Fatal(err)
			}

			dcControllerMockServer.RouteToHandler(
				"POST", communityMeterDefsPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", communityMeterDefsPath),
					ghttp.RespondWithPtr(&Status200, &communityMeterDefsBody),
				))

			returnedSystemMeterDefSlice := []marketplacev1beta1.MeterDefinition{*systemMeterDef1.DeepCopy()}
			systemMeterDefBody, err := json.Marshal(returnedSystemMeterDefSlice)
			if err != nil {
				log.Fatal(err)
			}

			dcControllerMockServer.RouteToHandler(
				"POST", systemMeterdefsPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", systemMeterdefsPath),
					ghttp.RespondWithPtr(&Status200, &systemMeterDefBody),
				))
		})

		It("Should create community defs if listed in the catalog", func() {
			Eventually(func() string {
				found := &marketplacev1beta1.MeterDefinition{}
				k8sClient.Get(context.TODO(), meterDef1Key, found)
				return found.Name
			}, timeout, interval).Should(Equal(meterDef1Key.Name))
		})

		It("Should create a system meterdef", func() {
			Eventually(func() string {
				foundSystemMeterDef := &marketplacev1beta1.MeterDefinition{}
				k8sClient.Get(context.TODO(), systemMeterDef1Key, foundSystemMeterDef)
				return foundSystemMeterDef.Name
			}, timeout, interval).Should(Equal(systemMeterDef1.Name))
		})
	})
})
