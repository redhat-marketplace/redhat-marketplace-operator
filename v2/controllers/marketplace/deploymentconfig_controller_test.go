package marketplace

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"os"

	// semver "github.com/blang/semver/v4"
	"github.com/blang/semver"
	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"

	// . "github.com/onsi/gomega/gstruct"
	osappsv1 "github.com/openshift/api/apps/v1"
	osimagev1 "github.com/openshift/api/image/v1"

	"github.com/operator-framework/api/pkg/lib/version"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	// marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	// marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = FDescribe("DeploymentConfig Controller Test", func() {

	var (
		csvName = "test-csv-1.v0.0.1"
		csvSplitName = "test-csv-1"
		namespace = "default"		
		// dcServer *ghttp.Server

	)

	meterDef1Key := types.NamespacedName{
		Name: "test-csv-1-meterdef",
		Namespace: namespace,
	}

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
				semver.Version{
					Major: 0,
					Minor: 0,
					Patch: 1,
				},
			},
		},
		Status: olmv1alpha1.ClusterServiceVersionStatus{},
	}

	dc := &osappsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.DEPLOYMENT_CONFIG_NAME,
			Namespace: "default",
		},
		Spec: osappsv1.DeploymentConfigSpec{
			Triggers: osappsv1.DeploymentTriggerPolicies{
				{
					Type: osappsv1.DeploymentTriggerOnConfigChange,	
					ImageChangeParams: &osappsv1.DeploymentTriggerImageChangeParams{
						Automatic: true,
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
					Type: osappsv1.DeploymentConditionType(osappsv1.DeploymentAvailable),
					Reason: "NewReplicationControllerAvailable",
					Status: corev1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					LastUpdateTime: metav1.Now(),
				},
			},
		},
	}

	is := &osimagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.DEPLOYMENT_CONFIG_NAME,
			Namespace: "default",
		},
		Spec:  osimagev1.ImageStreamSpec{
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
						Insecure: true,
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

	imageStreamID := "rhm-meterdefinition-file-server:v1"
	imageStreamTag := "v1"
	Status200 := 200
	listMeterDefsForCsvPath := "/" + catalog.ListForVersionEndpoint + "/" + csvSplitName + "/" + "0.0.1" + "/" + "default"
	systemMeterdefsPath := "/" + catalog.GetSystemMeterdefinitionTemplatesEndpoint


	BeforeEach(func(){
		os.Setenv("IMAGE_STREAM_ID",imageStreamID)
		os.Setenv("IMAGE_STREAM_TAG",imageStreamTag)
		// dcServer = ghttp.NewUnstartedServer()
		// dcServer.HTTPTestServer.URL = "127.0.0.1:64636"
		// fmt.Println("SERVER ADDR",dcServer.Addr())
		// dcServer = ghttp.NewServer()
		// dcServer.SetAllowUnhandledRequests(true)
		dcControllerMockServer.Start()
	})

	AfterEach(func(){
		csv := &olmv1alpha1.ClusterServiceVersion{}
		Expect(k8sClient.Get(context.TODO(),types.NamespacedName{Name:csvName,Namespace: "default"},csv)).Should(Succeed(),"get csv")
		Expect(k8sClient.Delete(context.TODO(), csv)).Should(Succeed(),"delete csv")

		_dc := &osappsv1.DeploymentConfig{}
		Expect(k8sClient.Get(context.TODO(),types.NamespacedName{Name:"rhm-meterdefinition-file-server",Namespace: "default"},_dc)).Should(Succeed(),"get deployment config")
		Expect(k8sClient.Delete(context.TODO(), _dc)).Should(Succeed(),"delete deploymentconfig")

		_is := &osimagev1.ImageStream{}
		Expect(k8sClient.Get(context.TODO(),types.NamespacedName{Name:"rhm-meterdefinition-file-server",Namespace: "default"},_is)).Should(Succeed(),"get image stream")
		Expect(k8sClient.Delete(context.TODO(), _is)).Should(Succeed(),"delete image stream")
		// dcServer.Close()
	})

		Context("Install community meterdefs for a particular csv if no community meterdefs are on the cluster",func() {
			BeforeEach(func(){
				Expect(k8sClient.Create(context.TODO(), csvOnCluster)).Should(Succeed(),"create test csv")
				Expect(k8sClient.Create(context.TODO(), dc)).Should(Succeed(),"create test deploymentconfig")
				Expect(k8sClient.Create(context.TODO(), is)).Should(Succeed(),"create test image stream")

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
								"marketplace.redhat.com/installedOperatorNameTag": "test-csv-1",
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
										common.GroupVersionKind{
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
	
				templatedMeterdefBody,err := json.Marshal(returnedTemplatedMeterdefSlice)
				if err != nil{
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
					"marketplace.redhat.com/isCommunityMeterdefintion": "true"
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
							Name:      "test-csv-1-meterdef",
							Namespace: "default",
							Annotations: map[string]string{
								"versionRange": "<=0.0.1",
							},
							Labels: map[string]string{
								"marketplace.redhat.com/installedOperatorNameTag": "test-csv-1",
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
										common.GroupVersionKind{
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

				meterdefForCsvBody,err := json.Marshal(returnedMeterdefGoSlice)
				if err != nil{
					log.Fatal(err)
				}

				dcControllerMockServer.RouteToHandler(
				"GET", listMeterDefsForCsvPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", listMeterDefsForCsvPath),
					ghttp.RespondWithPtr(&Status200, &meterdefForCsvBody),
				))
			})

			AfterEach(func(){
				// Expect(k8sClient.Delete(context.TODO(), csvOnCluster)).Should(Succeed(),"delete csv")
				// dcServer.Close()
			})

			It("Should find meterdef for test-csv-1",func(){
				Eventually(func() string {
					found := &marketplacev1beta1.MeterDefinition{}
					k8sClient.Get(context.TODO(), meterDef1Key, found)
					return found.Name
				}, timeout, interval).Should(Equal(meterDef1Key.Name))
			})
		})

		// Context("Update a community meterdefinition on the cluster",func() {
		// 	fmt.Println("UPDATE TEST")
		// 	BeforeEach(func(){
		// 		dcServer.Start()
		// 		meterdefOnCluster := &marketplacev1beta1.MeterDefinition{}
		// 		Expect(k8sClient.Get(context.TODO(), meterDef1Key, meterdefOnCluster)).Should(Succeed(),"find the existing community meterdefinition on the cluster")
				
		// 		returnedMeterdefGoSlice := []marketplacev1beta1.MeterDefinition{
		// 			{	
		// 				TypeMeta: metav1.TypeMeta{
		// 					Kind:       "MeterDefinition",
		// 					APIVersion: "marketplace.redhat.com/v1beta1",
		// 				},
		// 				ObjectMeta: metav1.ObjectMeta{
		// 					Name:      "test-csv-1-meterdef",
		// 					Namespace: "default",
		// 					Annotations: map[string]string{
		// 						"versionRange": "<=0.0.1",
		// 					},
		// 					Labels: map[string]string{
		// 						"marketplace.redhat.com/installedOperatorNameTag": "test-csv-1",
		// 						"marketplace.redhat.com/isCommunityMeterdefintion": "1",
		// 					},
		// 				},
		// 				Spec: marketplacev1beta1.MeterDefinitionSpec{
		// 					Group: "apps.partner.metering.com",
		// 					Kind:  "App",
		// 					ResourceFilters: []marketplacev1beta1.ResourceFilter{
		// 						{
		// 							WorkloadType: marketplacev1beta1.WorkloadTypeService,
		// 							OwnerCRD: &marketplacev1beta1.OwnerCRDFilter{
		// 								common.GroupVersionKind{
		// 									APIVersion: "test_package_1.com/v2",
		// 									Kind:       "test_package_1Cluster",
		// 								},
		// 							},
		// 							Namespace: &marketplacev1beta1.NamespaceFilter{
		// 								UseOperatorGroup: true,
		// 							},
		// 						},
		// 					},
		// 					Meters: []marketplacev1beta1.MeterWorkload{
		// 						{
		// 							//UPDATE
		// 							Name: "updated",
		// 							Aggregation: "sum",
		// 							GroupBy:     []string{"namespace"},
		// 							Period: &metav1.Duration{
		// 								Duration: time.Duration(time.Hour * 1),
		// 							},
		// 							Query:        "kube_service_labels{}",
		// 							Metric:       "test_package_1_cluster_count",
		// 							WorkloadType: marketplacev1beta1.WorkloadTypeService,
		// 							Without:      []string{"label_test_package_1_cluster", "label_app", "label_operator_test_package_1_com_version"},
		// 						},
		// 					},	
		// 				},	
		// 			},
		// 		}

		// 		meterdefForCsvBody,err := json.Marshal(returnedMeterdefGoSlice)
		// 		if err != nil{
		// 			log.Fatal(err)
		// 		}

		// 		dcServer.RouteToHandler(
		// 		"GET", listMeterDefsForCsvPath, ghttp.CombineHandlers(
		// 			ghttp.VerifyRequest("GET", listMeterDefsForCsvPath),
		// 			ghttp.RespondWithPtr(&Status200, &meterdefForCsvBody),
		// 		))			
		// 	})

		// 	It("meterdefintion on cluster should be updated",func(){
		// 		fmt.Println("DEPLIOYMENT CONFIG TEST")

		// 		Eventually(func() string {
		// 			found := &marketplacev1beta1.MeterDefinition{}
		// 			k8sClient.Get(context.TODO(), meterDef1Key, found)
		// 			return found.Spec.Meters[0].Name
		// 		}, timeout, interval).Should(Equal("updated"))
		// 	})

		// 	AfterEach(func(){
		// 		// Expect(k8sClient.Delete(context.TODO(), csvOnCluster)).Should(Succeed())
		// 		// Expect(k8sClient.Delete(context.TODO(), dc)).Should(Succeed())
		// 		// Expect(k8sClient.Delete(context.TODO(), is)).Should(Succeed())
		// 		dcServer.Close()
		// 	})
		// })
})
