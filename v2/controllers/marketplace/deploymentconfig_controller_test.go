package marketplace

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	// . "github.com/onsi/gomega/gstruct"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Remote resource S3 controller", func() {

	var (
		csvName = "test-csv-1"
		namespace = "openshift-redhat-marketplace"
	)

	// csvKey := types.NamespacedName{
	// 	Name:      csvName,
	// 	Namespace: namespace,
	// }

	meterDef1Key := types.NamespacedName{
		Name: "test-csv-1-meterdef",
		Namespace: namespace,
	}

	BeforeEach(func(){
		// csv1 := &olmv1alpha1.ClusterServiceVersion{
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name:      csvName,
		// 		Namespace: namespace,
		// 	},
		// }
	})

	Context("Install community meterdefs for a particular csv if no community meterdefs are on the cluster",func() {
		csv1 := &olmv1alpha1.ClusterServiceVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      csvName,
				Namespace: namespace,
			},
		}

		BeforeEach(func(){
			Expect(k8sClient.Create(context.TODO(), csv1)).Should(Succeed())

			server := ghttp.NewTLSServer()
			server.SetAllowUnhandledRequests(true)

			// os.Setenv("CATALOG_URL", server.Addr())
		

			statusCode := 200

			indexLabelsPath := "/" + catalog.GetMeterdefinitionIndexLabelEndpoint
		
			indexLabelsBody := []byte(`{
				"marketplace.redhat.com/installedOperatorNameTag": "test-csv-1",
				"marketplace.redhat.com/isCommunityMeterdefintion": "true"
			  }`)
	
			server.RouteToHandler(
				"GET", indexLabelsPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", indexLabelsPath,csvName),
					ghttp.RespondWithPtr(&statusCode, &indexLabelsBody),
				))

			listMeterDefsForCsvPath := "/" + catalog.ListForVersionEndpoint

			returnedMeterdefsBody := []byte(`{
				"kind": "MeterDefinition",
				"apiVersion": "marketplace.redhat.com/v1beta1",
				"metadata": {
				  "name": "test-csv-1-meterdef",
				  "namespace": "openshift-redhat-marketplace",
				  "labels": {
					"marketplace.redhat.com/installedOperatorNameTag": "test-csv-1",
					"marketplace.redhat.com/isCommunityMeterdefintion": "1"
				  },
				  "annotations": {
					"versionRange": "<=0.0.1"
				  }
				},
				"spec": {
				  "group": "apps.partner.metering.com",
				  "kind": "App",
				}
			  }`)

			server.RouteToHandler(
				"GET", listMeterDefsForCsvPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", listMeterDefsForCsvPath,namespace),
					ghttp.RespondWithPtr(&statusCode, &returnedMeterdefsBody),
				))
		})

		It("Should find meterdef for test-csv-1",func(){
			testMeterdef1 := &marketplacev1alpha1.MeterDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      csvName,
					Namespace: namespace,
					Labels: map[string]string{
						"marketplace.redhat.com/installedOperatorNameTag": "test-csv-1",
						"marketplace.redhat.com/isCommunityMeterdefintion": "1",
					},
					Annotations: map[string]string{
						"versionRange": "<=0.0.1",
					},
				},

				Spec: marketplacev1alpha1.MeterDefinitionSpec{
					Group: "apps.partner.metering.com",
					Kind:  "App",
				},
			}

			k8sClient.Get(context.TODO(), meterDef1Key, testMeterdef1)
			// csv1 := &olmv1alpha1.ClusterServiceVersion{
			// 	ObjectMeta: metav1.ObjectMeta{
			// 	Name:      csvName,
			// 	Namespace: namespace,
			// 	},
			// }
		})

	})

})
