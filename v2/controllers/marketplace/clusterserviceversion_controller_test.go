// Copyright 2020 IBM Corp.
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
	"fmt"

	"github.com/gotidy/ptr"
	. "github.com/onsi/gomega/gstruct"

	// . "github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/rectest"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = FDescribe("ClusterServiceVersion controller", func() {
	idFn := func(element interface{}) string {
		return fmt.Sprintf("%v", element)
	}

	var (
		csvName = "new-clusterserviceversion"
		subName = "new-subscription"

		clusterserviceversion = &olmv1alpha1.ClusterServiceVersion{
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
			},
		}
		subscription = &olmv1alpha1.Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      subName,
				Namespace: namespace,
				Labels: map[string]string{
					utils.OperatorTag: "true",
				},
			},
			Spec: &olmv1alpha1.SubscriptionSpec{},
			Status: olmv1alpha1.SubscriptionStatus{
				InstalledCSV: csvName,
			},
		}

		subscriptionWithoutLabels = &olmv1alpha1.Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      subName,
				Namespace: namespace,
			},
			Spec: &olmv1alpha1.SubscriptionSpec{},
			Status: olmv1alpha1.SubscriptionStatus{
				InstalledCSV: csvName,
			},
		}

		subscriptionDifferentCSV = &olmv1alpha1.Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      subName,
				Namespace: namespace,
				Labels: map[string]string{
					utils.OperatorTag: "true",
				},
			},
			Spec: &olmv1alpha1.SubscriptionSpec{},
			Status: olmv1alpha1.SubscriptionStatus{
				InstalledCSV: "dummy",
			},
		}
	)

	var empty map[string]string

	DescribeTable("filter events",
		func(annotations map[string]string, labels map[string]string, expected bool) {
			obj := &metav1.ObjectMeta{}
			obj.SetAnnotations(annotations)
			obj.SetLabels(labels)
			Expect(csvFilter(obj)).To(Equal(expected))
		},
		Entry("deny mdef with copied from", map[string]string{
			utils.CSV_METERDEFINITION_ANNOTATION: "some meterdef",
		}, map[string]string{
			olmCopiedFromTag: "foo",
		}, false),
		Entry("accept mdef without copied from", map[string]string{
			utils.CSV_METERDEFINITION_ANNOTATION: "some meterdef",
			"olm.operatorNamespace":              "default",
		}, empty, true),
	)

	Context("predicates", func() {
		It("should deny delete events", func() {
			evt := event.DeleteEvent{}
			Expect(clusterServiceVersionPredictates.Delete(evt)).To(BeFalse())
		})
		It("should deny generic events", func() {
			evt := event.GenericEvent{}
			Expect(clusterServiceVersionPredictates.GenericFunc(evt)).To(BeFalse())
		})
		It("should check for change in mdef", func() {
			evt := event.UpdateEvent{}
			evt.ObjectNew = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						utils.CSV_METERDEFINITION_ANNOTATION: "newmdef",
						"olm.operatorNamespace":              "default",
					},
				},
			}
			evt.ObjectOld = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						utils.CSV_METERDEFINITION_ANNOTATION: "oldmdef",
						"olm.operatorNamespace":              "default",
					},
				}}
			Expect(clusterServiceVersionPredictates.Update(evt)).To(BeTrue())
			evt.ObjectOld.GetAnnotations()[utils.CSV_METERDEFINITION_ANNOTATION] = "newmdef"
			Expect(clusterServiceVersionPredictates.Update(evt)).To(BeFalse())
		})
	})

	Context("controller filtering of csvs", func() {
		AfterEach(func() {
			subscription := &olmv1alpha1.Subscription{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      subName,
				Namespace: operatorNamespace,
			}, subscription)).Should(Succeed(), "get subscription")
			k8sClient.Delete(context.TODO(), subscription)

			csv := &olmv1alpha1.ClusterServiceVersion{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      csvName,
				Namespace: operatorNamespace,
			}, csv)).Should(Succeed(), "get csv")
			k8sClient.Delete(context.TODO(), csv)
		})

		It("List csvs", func() {
			Expect(k8sClient.Create(context.TODO(), subscription.DeepCopy())).Should(Succeed(), "create subscription")
			Expect(k8sClient.Create(context.TODO(), clusterserviceversion.DeepCopy())).Should(Succeed(), "create csv")

			Eventually(func() []string {
				csvList := &olmv1alpha1.ClusterServiceVersionList{}
				k8sClient.List(context.TODO(), csvList)

				var csvNames []string
				for _, mdef := range csvList.Items {
					csvNames = append(csvNames, mdef.Name)
				}

				return csvNames
			}, timeout, interval).Should(And(
				HaveLen(1),
				MatchAllElements(idFn, Elements{
					"new-clusterserviceversion": Equal("new-clusterserviceversion"),
				}),
			))
		})

		It("Should not process mismatched csv and subscriptions", func() {
			Expect(k8sClient.Create(context.TODO(), subscriptionDifferentCSV.DeepCopy())).Should(Succeed(), "create subscription")
			Expect(k8sClient.Create(context.TODO(), clusterserviceversion.DeepCopy())).Should(Succeed(), "create subscription")

			Eventually(func() []string {
				csvList := &olmv1alpha1.ClusterServiceVersionList{}
				labels := map[string]string{
					watchTag: "lite",
				}
				listOpts := []client.ListOption{
					client.MatchingLabels(labels),
				}
				k8sClient.List(context.TODO(), csvList, listOpts...)

				var csvNames []string
				for _, mdef := range csvList.Items {
					csvNames = append(csvNames, mdef.Name)
				}

				return csvNames
			}, timeout, interval).Should(And(
				HaveLen(0),
			))
		})

		It("Should not process subscriptions without labels", func() {
			Expect(k8sClient.Create(context.TODO(), subscriptionWithoutLabels.DeepCopy())).Should(Succeed(), "create subscription")
			Expect(k8sClient.Create(context.TODO(), clusterserviceversion.DeepCopy())).Should(Succeed(), "create subscription")

			Eventually(func() []string {
				csvList := &olmv1alpha1.ClusterServiceVersionList{}
				labels := map[string]string{
					watchTag: "lite",
				}
				listOpts := []client.ListOption{
					client.MatchingLabels(labels),
				}
				k8sClient.List(context.TODO(), csvList, listOpts...)

				var csvNames []string
				for _, mdef := range csvList.Items {
					csvNames = append(csvNames, mdef.Name)
				}

				return csvNames
			}, timeout, interval).Should(And(
				HaveLen(0),
			))
		})
	})

	It("BuildMeterDefinitionFromString", func() {
		meter := &marketplacev1alpha1.MeterDefinition{}
		name := "example-meterdefinition"
		group := "partner.metering.com"
		version := "v1alpha"
		kind := "App"
		var ann = map[string]string{
			utils.CSV_ANNOTATION_NAME:      csvName,
			utils.CSV_ANNOTATION_NAMESPACE: namespace,
		}

		ogMeter := &marketplacev1alpha1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   namespace,
				Annotations: ann,
			},
			Spec: marketplacev1alpha1.MeterDefinitionSpec{
				Group:   group,
				Version: ptr.String(version),
				Kind:    kind,
			},
		}

		meterStr, _ := json.Marshal(ogMeter)

		Expect(meter.BuildMeterDefinitionFromString(string(meterStr), csvName, namespace, utils.CSV_ANNOTATION_NAME, utils.CSV_ANNOTATION_NAMESPACE)).Should(Succeed(), "build meterdefinition from string")
	})
})
