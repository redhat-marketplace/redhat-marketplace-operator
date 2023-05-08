// Copyright 2022 IBM Corp.
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

package filter

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
)

var _ = Describe("lookup", func() {
	var sut *MeterDefinitionLookupFilter

	BeforeEach(func() {
		restMapper, _ := managers.NewDynamicRESTMapper(cfg)
		metadataInterface, _ := metadata.NewForConfig(cfg)
		metadataClient := client.NewMetadataClient(metadataInterface, restMapper)

		ac := client.NewAccessChecker(&kubernetes.Clientset{}, context.TODO())

		sut = &MeterDefinitionLookupFilter{
			client:    k8sClient,
			findOwner: client.NewFindOwnerHelper(context.TODO(), metadataClient, ac),
		}
		sut.log = logr.Discard()
	})

	It("should create label filters", func() {
		sut.MeterDefinition = &v1beta1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "foobar",
			},
			Spec: v1beta1.MeterDefinitionSpec{
				ResourceFilters: []v1beta1.ResourceFilter{
					{
						Namespace: &v1beta1.NamespaceFilter{
							UseOperatorGroup: true,
						},
						Label: &v1beta1.LabelFilter{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"a.b.c/a": "a",
								},
							},
						},
						WorkloadType: common.WorkloadTypePod,
					},
				},
			},
		}

		filters, err := sut.createFilters(sut.MeterDefinition)
		Expect(err).To(Succeed())
		Expect(filters).To(HaveLen(1))
		Expect(filters[0]).To(HaveLen(3))
		Expect(filters[0].Namespaces()).To(ConsistOf("foobar"))
		Expect(filters[0].Types()).To(ConsistOf(reflect.TypeOf(&corev1.Pod{})))

		types := []reflect.Type{}
		for _, filter := range filters[0] {
			types = append(types, reflect.TypeOf(filter))
		}
		Expect(types).To(ConsistOf(
			reflect.TypeOf(&WorkloadLabelFilter{}),     // should have a label filter
			reflect.TypeOf(&WorkloadNamespaceFilter{}), // a namespace filter
			reflect.TypeOf(&WorkloadTypeFilter{}),      // workload type filter
		))

	})

	It("should find namespace fallbacks properly", func() {
		sut.MeterDefinition = &v1beta1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "openshift-operators",
			},
			Spec: v1beta1.MeterDefinitionSpec{
				ResourceFilters: []v1beta1.ResourceFilter{
					{
						Namespace: &v1beta1.NamespaceFilter{
							UseOperatorGroup: true,
						},
						Label: &v1beta1.LabelFilter{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"a.b.c/a": "a",
								},
							},
						},
						WorkloadType: common.WorkloadTypePod,
					},
				},
			},
		}

		filters, err := sut.createFilters(sut.MeterDefinition)
		Expect(err).To(Succeed())
		Expect(filters).To(HaveLen(1))
		Expect(filters[0]).To(HaveLen(3))
		Expect(filters[0].Namespaces()).To(ConsistOf(""))
		Expect(filters[0].Types()).To(ConsistOf(reflect.TypeOf(&corev1.Pod{})))
	})

	Context("operator groups", func() {
		var og = olmv1.OperatorGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "og1",
				Namespace: "a",
			},
			Spec: olmv1.OperatorGroupSpec{
				TargetNamespaces: []string{
					"a",
				},
			},
			Status: olmv1.OperatorGroupStatus{
				Namespaces: []string{"a"},
			},
		}

		var pod = corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "a",
				Labels: map[string]string{
					"a.b.c/a": "a",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "foo",
						Image: "foo",
					},
				},
			},
		}

		BeforeEach(func() {
			k8sClient.Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "a",
				},
			})
			Expect(k8sClient.Create(context.TODO(), &og)).To(Succeed())
			now := metav1.Now()
			og.Status = olmv1.OperatorGroupStatus{
				Namespaces:  []string{"a"},
				LastUpdated: &now,
			}
			Expect(k8sClient.Status().Update(context.TODO(), &og)).To(Succeed())
			Expect(k8sClient.Create(context.TODO(), &pod)).To(Succeed())
		})

		AfterEach(func() {
			k8sClient.Delete(context.TODO(), &og)
			k8sClient.Delete(context.TODO(), &pod)
		})

		It("should use operator groups successfully", func() {
			sut.MeterDefinition = &v1beta1.MeterDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "a",
				},
				Spec: v1beta1.MeterDefinitionSpec{
					ResourceFilters: []v1beta1.ResourceFilter{
						{
							Namespace: &v1beta1.NamespaceFilter{
								UseOperatorGroup: true,
							},
							Label: &v1beta1.LabelFilter{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"a.b.c/a": "a",
									},
								},
							},
							WorkloadType: common.WorkloadTypePod,
						},
					},
				},
			}

			filters, err := sut.createFilters(sut.MeterDefinition)
			Expect(err).To(Succeed())
			Expect(filters).To(HaveLen(1))
			Expect(filters[0]).To(HaveLen(3))
			Expect(filters[0].Namespaces()).To(ConsistOf("a"))
			Expect(filters[0].Types()).To(ConsistOf(reflect.TypeOf(&corev1.Pod{})))

			ans, i, err := filters[0].Test(&pod)
			Expect(err).To(Succeed())
			Expect(i).To(Equal(-1))
			Expect(ans).To(BeTrue())
		})
	})
})
