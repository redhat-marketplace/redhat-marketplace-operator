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

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/mock/mock_client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/utils/pointer"
	k8client "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("JsonMeterDefValidation", func() {
	var (
		ctrl   *gomock.Controller
		client *mock_client.MockClient
		//statusWriter *mock_client.MockStatusWriter
		ctx context.Context
		//patcher      *mock_patch.MockPatchMaker
		sut *ClusterServiceVersionReconciler
		//cc  reconcileutils.ClientCommandRunner
		meterDefStatus = "marketplace.redhat.com/meterDefinitionStatus"
		meterDefError  = "marketplace.redhat.com/meterDefinitionError"
		CSV            = &olmv1alpha1.ClusterServiceVersion{}
	)

	BeforeEach(func() {
		logger := logf.Log.WithName("JsonMeterDefValidation")
		ctrl = gomock.NewController(GinkgoT())
		//patcher = mock_patch.NewMockPatchMaker(ctrl)
		client = mock_client.NewMockClient(ctrl)
		//statusWriter = mock_client.NewMockStatusWriter(ctrl)
		marketplacev1alpha1.AddToScheme(scheme.Scheme)
		olmv1alpha1.AddToScheme(scheme.Scheme)
		//cc = reconcileutils.NewClientCommand(client, scheme.Scheme, logger)
		ctx = context.TODO()

		sut = &ClusterServiceVersionReconciler{Client: client, Scheme: scheme.Scheme, Log: logger}

		CSV = &olmv1alpha1.ClusterServiceVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "n",
				Namespace: "ns",
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("MeterDefinition Json String", func() {
		var (
			meterDefJsonBad = `{
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
				ownerCRD": {
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
			annBad       = map[string]string{utils.CSV_METERDEFINITION_ANNOTATION: meterDefJsonBad}
			meterDefJson = `{
        "apiVersion": "marketplace.redhat.com/v1alpha1",
        "kind": "MeterDefinition",
        "metadata": {
          "name": "robinstorage-meterdef"
        },
        "spec": {
          "meterGroup": "robinclusters.robin.io",
          "meterKind": "RobinCluster",
          "workloadVertexType": "OperatorGroup",
          "workloads": [{
            "name": "pod_node",
            "type": "Pod",
            "ownerCRD": {
                "apiVersion": "manage.robin.io/v1",
                "kind": "RobinCluster"
            },
            "metricLabels": [{
                "aggregation": "avg",
                "label": "node_hour2",
                "query": "min_over_time((kube_pod_info{created_by_kind=\"DaemonSet\",created_by_name=\"robin\",node=~\".*\"} or on() vector(0))[60m:60m])"
            }]
          }]
        }
      }`
			ann             = map[string]string{utils.CSV_METERDEFINITION_ANNOTATION: meterDefJson}
			meterDefinition *marketplacev1alpha1.MeterDefinition

			//meterDefinitionNew   *marketplacev1alpha1.MeterDefinition
			gvk *schema.GroupVersionKind
		)

		BeforeEach(func() {
			meterDefinition = &marketplacev1alpha1.MeterDefinition{}

			//meterDefinitionNew = nil
			gvk = &schema.GroupVersionKind{}
			gvk.Kind = "ClusterServiceVersion"
			gvk.Version = "operators.coreos.com/v1alpha1"
			meterDefinition.SetNamespace("test-namespace")
			CSV.SetNamespace("test-namespace")
			CSV.SetName("mock-csv")
			_ = meterDefinition.BuildMeterDefinitionFromString(
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
			list := v1beta1.MeterDefinitionList{
				Items: []v1beta1.MeterDefinition{},
			}
			client.EXPECT().Update(ctx, CSV).Return(nil).Times(2)
			client.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, l *v1beta1.MeterDefinitionList, _ k8client.ListOption) error {
					*l = list
					return nil
				}).Times(1)
			client.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, mdef *v1beta1.MeterDefinition) error {

				Expect(mdef.Name).To(Equal("robinstorage-meterdef"))
				return nil
			}).Times(1)

			By("testing for failure")
			sut.reconcileMeterDefAnnotation(CSV, annBad)
			Expect(CSV.GetAnnotations()[meterDefStatus]).To(Equal("error"))
			Expect(CSV.GetAnnotations()[meterDefError]).Should(ContainSubstring("invalid character"))

			//client.EXPECT().Update(ctx, CSV).Return(nil).Times(1)
			By("testing for success")
			sut.reconcileMeterDefAnnotation(CSV, ann)
			Expect(CSV.GetAnnotations()[meterDefStatus]).To(Equal("success"))
			Expect(CSV.GetAnnotations()[meterDefError]).Should(BeEmpty())

		})
	})
})
