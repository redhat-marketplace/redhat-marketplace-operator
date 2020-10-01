package clusterserviceversion

import (
	"context"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/mock/mock_client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/utils/pointer"
	k8client "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("JsonMeterDefValidation", func() {
	var (
		ctrl   *gomock.Controller
		client *mock_client.MockClient
		//statusWriter *mock_client.MockStatusWriter
		ctx context.Context
		//patcher      *mock_patch.MockPatchMaker
		sut *ReconcileClusterServiceVersion
		//cc  reconcileutils.ClientCommandRunner
		meterDefStatus = "marketplace.redhat.com/meterDefStatus"
		meterDefError  = "marketplace.redhat.com/meterDefError"
		CSV            = &olmv1alpha1.ClusterServiceVersion{}
	)

	BeforeEach(func() {
		//logger := logf.Log.WithName("JsonMeterDefValidation")
		ctrl = gomock.NewController(GinkgoT())
		//patcher = mock_patch.NewMockPatchMaker(ctrl)
		client = mock_client.NewMockClient(ctrl)
		//statusWriter = mock_client.NewMockStatusWriter(ctrl)
		apis.AddToScheme(scheme.Scheme)
		olmv1alpha1.AddToScheme(scheme.Scheme)
		//cc = reconcileutils.NewClientCommand(client, scheme.Scheme, logger)
		ctx = context.TODO()

		sut = &ReconcileClusterServiceVersion{client: client, scheme: scheme.Scheme}

	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("MeterDefinition Json String is wrong", func() {
		var (
			meterDefJson = `{
			"apiVersion": "marketplace.redhat.com/v1alpha1",
			"kind": "MeterDefinition",
			"metadata": {
			  "name": "robinstorage-meterdef"
			},
			"spec": {
			  "group": "robinclusters.robin.io",
			  "kind": "RobinCluster",
			  "workloadVertexType": "OperatorGroup",
			  "workloads": {
				"name": "pod_node",
				"type": "Pod",
				"ownerCRD": {
					"apiVersion": "robin.io/v1alpha1",
					"kind": "RobinCluster"
				},
				"metricLabels": [{
					"aggregation": "sum",
					"label": "node_hour",
					"query": "kube_pod_info{created_by_kind=\"DaemonSet\",created_by_name=\"robin\"}"
				}]
			  }
			}
		  }`
			ann = map[string]string{utils.CSV_METERDEFINITION_ANNOTATION: meterDefJson}
		)

		It("CSV should have annotation meterDefStatus with value 'error' and meterDefError", func() {
			client.EXPECT().Update(ctx, CSV).Return(nil).Times(1)
			sut.reconcileMeterDefAnnotation(CSV, ann)
			Expect(CSV.GetAnnotations()[meterDefStatus]).To(Equal("error"))
			Expect(CSV.GetAnnotations()[meterDefError]).Should(ContainSubstring("cannot unmarshal object"))
		})
	})

	Context("MeterDefinition Json String is correct format", func() {
		var (
			meterDefJson = `{
					"apiVersion": "marketplace.redhat.com/v1alpha1",
					"kind": "MeterDefinition",
					"metadata": {
					  "name": "robinstorage-meterdef"
					},
					"spec": {
					  "group": "robinclusters.robin.io",
					  "kind": "RobinCluster",
					  "workloadVertexType": "OperatorGroup",
					  "workloads": [{
						"name": "pod_node",
						"type": "Pod",
						"ownerCRD": {
							"apiVersion": "robin.io/v1alpha1",
							"kind": "RobinCluster"
						},
						"metricLabels": [{
							"aggregation": "sum",
							"label": "node_hour",
							"query": "kube_pod_info{created_by_kind=\"DaemonSet\",created_by_name=\"robin\"}"
						}]
					  }]
					}
				  }`
			ann                  = map[string]string{utils.CSV_METERDEFINITION_ANNOTATION: meterDefJson}
			trackmeterAnnotation = map[string]string{trackMeterTag: "true"}
			list                 *marketplacev1alpha1.MeterDefinitionList
			meterDefinition      *marketplacev1alpha1.MeterDefinition

			//meterDefinitionNew   *marketplacev1alpha1.MeterDefinition
			gvk *schema.GroupVersionKind
		)
		BeforeEach(func() {
			list = &marketplacev1alpha1.MeterDefinitionList{}
			meterDefinition = &marketplacev1alpha1.MeterDefinition{}

			//meterDefinitionNew = nil
			gvk = &schema.GroupVersionKind{}
			gvk.Kind = "ClusterServiceVersion"
			gvk.Version = "operators.coreos.com/v1alpha1"
			meterDefinition.SetNamespace("test-namespace")
			CSV.SetNamespace("test-namespace")
			CSV.SetName("mock-csv")
			CSV.SetAnnotations(trackmeterAnnotation)
			_, _ = meterDefinition.BuildMeterDefinitionFromString(
				meterDefJson,
				CSV.GetName(),
				CSV.GetNamespace(),
				utils.CSV_ANNOTATION_NAME,
				utils.CSV_ANNOTATION_NAMESPACE)
			ref := metav1.OwnerReference{
				APIVersion:         gvk.GroupVersion().String(),
				Kind:               gvk.Kind,
				Name:               CSV.GetName(),
				UID:                CSV.GetUID(),
				BlockOwnerDeletion: pointer.BoolPtr(false),
				Controller:         pointer.BoolPtr(false),
			}

			meterDefinition.ObjectMeta.OwnerReferences = append(meterDefinition.ObjectMeta.OwnerReferences, ref)
		})
		It("CSV should have annotation meterDefStatus with value 'error' and meterDefError", func() {
			client.EXPECT().List(ctx, list, k8client.InNamespace(meterDefinition.GetNamespace())).Return(nil).Times(1)
			client.EXPECT().Update(ctx, CSV).Return(nil).Times(2)
			client.EXPECT().Create(ctx, meterDefinition).Return(nil).Times(1)
			sut.reconcileMeterDefAnnotation(CSV, ann)
			Expect(CSV.GetAnnotations()[meterDefStatus]).To(Equal("success"))
			Expect(CSV.GetAnnotations()[meterDefError]).Should(BeEmpty())

		})
	})

})
