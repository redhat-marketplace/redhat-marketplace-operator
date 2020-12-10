package controller_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega/gstruct"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var meterDefName = "test"
var _ = Describe("MeterDefController reconcile", func() {
	BeforeEach(func() {
		Expect(testHarness.BeforeAll()).To(Succeed())
	})

	AfterEach(func() {
		Expect(testHarness.AfterAll()).To(Succeed())
		// clearMeterBaseLabels()
	})

	Context("Meterdefinition reconcile", func() {

		// var meterdef *v1alpha1.MeterDefinition

		BeforeEach(func(done Done) {
			meterdef := &v1alpha1.MeterDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      meterDefName,
					Namespace: Namespace,
				},
				Spec: v1alpha1.MeterDefinitionSpec{
					Group:              "marketplace.redhat.com",
					Kind:               "MetricState",
					WorkloadVertexType: v1alpha1.WorkloadVertexOperatorGroup,
					Workloads: []v1alpha1.Workload{
						{
							Name:         "test",
							WorkloadType: v1alpha1.WorkloadTypePod,
							OwnerCRD: &common.GroupVersionKind{
								APIVersion: "apps/v1",
								Kind:       "StatefulSet",
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

			// create the meterdef
			Eventually(func()bool{
				return Expect(testHarness.Create(context.TODO(), meterdef)).Should(SucceedOrAlreadyExist,"create the meterdef")
			},180).Should(BeTrue(),"Should create a meterdef if not found")
	
			//update the meterbase to trigger a requeue
			// updateMeterBaseLabels()
		
			// clear conditions on the meterdef
			// clearMeterDefConditions()

			// check for config map and prom service
			Eventually(func()bool{
				certConfigMap := &corev1.ConfigMap{}
				promService := &corev1.Service{}

				assertion := Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: utils.OPERATOR_CERTS_CA_BUNDLE_NAME, Namespace: Namespace}, certConfigMap)).Should(Succeed(),"find config map")
				assertion = Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: utils.PROMETHEUS_METERBASE_NAME, Namespace: Namespace}, promService)).Should(Succeed(),"find prom service")
				return assertion
			},300).Should(BeTrue(),"check for config map and prom service")
			close(done)
		}, 400)

		It("Should find a meterdef", func(done Done) {
			Eventually(func() bool {
				meterdef := &v1alpha1.MeterDefinition{}
				assertion := Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: meterDefName, Namespace: Namespace}, meterdef)).Should(Succeed(),"find a meterdef")
				return assertion
			}, 180).Should(BeTrue(),"should find a meterdef")
			close(done)
		}, 180)

		It("Should query prometheus and append metric data to meterdef status", func(done Done) {
			Eventually(func() bool {
				meterdef := &v1alpha1.MeterDefinition{}
				assertion := Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: meterDefName, Namespace: Namespace}, meterdef)).Should(Succeed())
				assertion = runAssertionOnMeterDef(meterdef)
				return assertion
			}, 500).Should(BeTrue())
			close(done)
		}, 500,30)

		// TODO: these probably better for the meterbase controller int test

		// Context("Error handling for operator-cert-ca-bundle config map - not found error", func() {
		// 	certConfigMap := &corev1.ConfigMap{
		// 		ObjectMeta: metav1.ObjectMeta{
		// 			Name:      utils.OPERATOR_CERTS_CA_BUNDLE_NAME,
		// 			Namespace: Namespace,
		// 		},
		// 	}
		// 	BeforeEach(func(done Done) {
		// 		Eventually(func() bool {
					
		// 			assertion := Expect(testHarness.Delete(context.TODO(), certConfigMap)).Should(Succeed(),"delete cert config map")
		// 			return assertion
		// 		}, 300).Should(BeTrue())

		// 		clearMeterDefConditions()
		// 		close(done)
		// 	}, 300)

		// 	It("Should log an error if the operator-cert-ca-bundle config map is not found", func(done Done) {
		// 		meterdef := &v1alpha1.MeterDefinition{}
		// 		Eventually(func() bool {
		// 			assertion := Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: meterDefName, Namespace: Namespace}, meterdef)).Should(Succeed())
		// 			assertion = runAssertionOnConditions(meterdef.Status, "Failed to retrieve operator-certs-ca-bundle.: ConfigMap \"operator-certs-ca-bundle\" not found")

		// 			return assertion

		// 		}, 300).Should(BeTrue())
		// 		close(done)
		// 	}, 300)
		// })

		// Context("Error handling for operator-cert-ca-bundle config map - key mis-match error", func() {
		// 	BeforeEach(func(done Done) {
		// 		Eventually(func() bool {
		// 			updatedCertConfigMap := &corev1.ConfigMap{
		// 				ObjectMeta: metav1.ObjectMeta{
		// 					Name: utils.OPERATOR_CERTS_CA_BUNDLE_NAME,
		// 					Namespace: Namespace,
		// 				},
		// 			}
		// 			updatedCertConfigMap.Data = map[string]string{
		// 				"wrong-key": "wrong-value",
		// 			}
		// 			assertion := Expect(testHarness.Upsert(context.TODO(),updatedCertConfigMap)).Should(Succeed())
		// 			assertion = Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: utils.OPERATOR_CERTS_CA_BUNDLE_NAME, Namespace: Namespace}, updatedCertConfigMap)).Should(Succeed())	
		// 			return assertion
		// 		}, 300).Should(BeTrue())
		// 		close(done)
		// 	}, 300)
			
			
		// 	It("Should log an error if the operator-cert-ca-bundle config map is misconfigured", func(done Done) {
		// 		meterdef := &v1alpha1.MeterDefinition{}
		// 		Eventually(func() bool {
		// 			assertion := Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: meterDefName, Namespace: Namespace}, meterdef)).Should(Succeed())
		// 			assertion = runAssertionOnConditions(meterdef.Status, "Error retrieving cert from config map")
		// 			return assertion

		// 		}, 300).Should(BeTrue())
		// 		close(done)
		// 	}, 300)
		// })

		// Context("Error handling for prometheus service", func() {
		// 	BeforeEach(func(done Done) {
		// 		promservice := corev1.Service{
		// 			ObjectMeta: metav1.ObjectMeta{
		// 				Name:      "rhm-prometheus-meterbase",
		// 				Namespace: Namespace,
		// 			},
		// 		}

		// 		Expect(testHarness.Delete(context.TODO(), &promservice)).Should(Succeed())

		// 		close(done)
		// 	}, 300)


		// 	It("Should log an error if the prometheus service isn't found", func(done Done) {
		// 		meterdef := &v1alpha1.MeterDefinition{}
		// 		Eventually(func() bool {
		// 			assertion := Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: meterDefName, Namespace: Namespace}, meterdef)).Should(Succeed())
		// 			errorMsg := "failed to get prometheus service: Service \"rhm-prometheus-meterbase\" not found"
		// 			assertion = runAssertionOnConditions(meterdef.Status, errorMsg)
		// 			return assertion
		// 		}, 300).Should(BeTrue())
		// 		close(done)
		// 	}, 300)
		// })
	})
})

func runAssertionOnConditions(status v1alpha1.MeterDefinitionStatus, errorMsg string) (assertion bool) {
	for _, condition := range status.Conditions {
		if condition.Message == errorMsg {
			assertion = true
			return assertion
		}
	}

	assertion = false
	return assertion
}

func runAssertionOnMeterDef(meterdef *v1alpha1.MeterDefinition) (assertion bool) {
	if meterdef.Status.Results != nil {
		if meterdef.Status.Results[0].Value > 0 {
			startTime := meterdef.Status.Results[0].StartTime
			endTime := meterdef.Status.Results[0].EndTime

			result := map[string]interface{}{
				"value":        meterdef.Status.Results[0].Value,
				"endTime":      meterdef.Status.Results[0].EndTime,
				"startTime":    meterdef.Status.Results[0].StartTime,
				"queryName":    meterdef.Status.Results[0].QueryName,
				"workloadName": meterdef.Status.Results[0].WorkloadName,
			}

			assertion = Expect(result).Should(MatchAllKeys(Keys{
				"workloadName": Equal("test"),
				"queryName":    Equal("test"),
				"startTime":    Equal(startTime),
				"endTime":      Equal(endTime),
				// depending on when this runs it could be 1 or 2
				"value": Not(BeZero()),
			}))
		}
	}

	return assertion
}

func GetOperatorPodObject()(bool,*corev1.Pod,string){
	listOpts := []client.ListOption{
        client.InNamespace(Namespace),
        client.MatchingLabels(map[string]string{
            "redhat.marketplace.com/name": "redhat-marketplace-operator",
        }),
    }

    var operatorPodName string
    podList := &corev1.PodList{}
    assertion := Expect(testHarness.List(context.TODO(),podList,listOpts...)).Should(Succeed())

    podNames := utils.GetPodNames(podList.Items)
    if len(podNames) == 1 {
		fmt.Println(podNames[0])
        operatorPodName = podNames[0]
    }
    
    operatorPod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name: operatorPodName,
            Namespace: Namespace,
            Labels: map[string]string{
                "redhat.marketplace.com/name": "redhat-marketplace-operator",
            },
        },
	}
	
	return assertion,operatorPod,operatorPodName
}

func IsRHMPodRunning(rhmPod *corev1.Pod) bool{
    if rhmPod.Status.ContainerStatuses == nil {
        return false
    }

    for _,containerStatus := range rhmPod.Status.ContainerStatuses{
        if containerStatus.Ready == true {
			fmt.Println("rhm container running")
            return true
        }
    }

    return false
}

func deleteOperatorPod() bool{
	assertion,operatorPod,_ := GetOperatorPodObject()
	assertion = Expect(testHarness.Delete(context.TODO(), operatorPod)).Should(Succeed())
	if assertion == true{
		fmt.Println("deleted operator pod, sleeping for 60 secs")
	}
  
    return assertion
}

func clearMeterDefConditions(){
	Eventually(func()bool{
		updatedMeterDef := &v1alpha1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: meterDefName,
				Namespace: Namespace,
			},
		}
		
		updatedMeterDef.Status.Conditions = nil
		assertion := Expect(testHarness.Upsert(context.TODO(),updatedMeterDef)).Should(Succeed(),"update to clear meterdef status")
		
		if assertion == true {
			fmt.Println("cleared conditions on meterdef")
		}
		
		return assertion
	},300).Should(BeTrue(),"reset conditions on the meterdef")
}

func updateMeterBaseLabels(){
	Eventually(func()bool{
		meterbase := &v1alpha1.MeterBase{}
		assertion := Expect(testHarness.Get(context.TODO(),types.NamespacedName{Name: utils.METERBASE_NAME,Namespace: Namespace},meterbase)).Should(Succeed())
		patch := client.MergeFrom(meterbase.DeepCopy())

		meterbase.Labels = map[string]string{
			"int-test" : "force-requeue",
		}

		assertion = Expect(testHarness.Patch(context.TODO(),meterbase,patch)).Should(Succeed())
		return assertion
	},120).Should(BeTrue())
}

func clearMeterBaseLabels(){
	Eventually(func()bool{
		meterbase := &v1alpha1.MeterBase{}
		assertion := Expect(testHarness.Get(context.TODO(),types.NamespacedName{Name: utils.METERBASE_NAME,Namespace: Namespace},meterbase)).Should(Succeed())
		patch := client.MergeFrom(meterbase.DeepCopy())

		meterbase.Labels = nil
		assertion = Expect(testHarness.Patch(context.TODO(),meterbase,patch)).Should(Succeed())
		return assertion
	},120).Should(BeTrue())
}