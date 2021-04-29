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
	"github.com/gotidy/ptr"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/rectest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	. "github.com/onsi/ginkgo/extensions/table"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ClusterServiceVersion controller", func() {
	var empty map[string]string

	DescribeTable("filter events",
		func(annotations map[string]string, labels map[string]string, expected int) {
			obj := &metav1.ObjectMeta{}
			obj.SetAnnotations(annotations)
			obj.SetLabels(labels)
			Expect(csvFilter(obj)).To(Equal(expected))
		},
		Entry("allow unignored csv", empty, empty, 2),
		Entry("deny ignored", map[string]string{
			ignoreTag: ignoreTagValue,
		}, empty, 0),
		Entry("deny mdef with copied from", map[string]string{
			olmCopiedFromTag:                     "foo",
			utils.CSV_METERDEFINITION_ANNOTATION: "some meterdef",
		}, empty, 0),
		Entry("accept mdef without copied from", map[string]string{
			utils.CSV_METERDEFINITION_ANNOTATION: "some meterdef",
		}, empty, 1),
	)

	Context("predicates", func() {
		It("should allow delete events", func() {
			evt := event.DeleteEvent{}
			Expect(clusterServiceVersionPredictates.Delete(evt)).To(BeTrue())
		})
		It("should deny generic events", func() {
			evt := event.GenericEvent{}
			Expect(clusterServiceVersionPredictates.GenericFunc(evt)).To(BeFalse())
		})
		It("should check for change in mdef", func() {
			evt := event.UpdateEvent{}
			evt.MetaNew = &metav1.ObjectMeta{
				Annotations: map[string]string{
					utils.CSV_METERDEFINITION_ANNOTATION: "newmdef",
				},
			}
			evt.MetaOld = &metav1.ObjectMeta{
				Annotations: map[string]string{
					utils.CSV_METERDEFINITION_ANNOTATION: "oldmdef",
				},
			}
			Expect(clusterServiceVersionPredictates.Update(evt)).To(BeTrue())
			evt.MetaOld.GetAnnotations()[utils.CSV_METERDEFINITION_ANNOTATION] = "newmdef"
			Expect(clusterServiceVersionPredictates.Update(evt)).To(BeFalse())
		})
	})

	It("cluster service version controller", func() {
		var (
			csvName   = "new-clusterserviceversion"
			subName   = "new-subscription"
			namespace = "arbitrary-namespace"

			req = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      csvName,
					Namespace: namespace,
				},
			}
			opts = []StepOption{
				WithRequest(req),
			}

			clusterserviceversion = &olmv1alpha1.ClusterServiceVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      csvName,
					Namespace: namespace,
				},
			}
			subscription = &olmv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      subName,
					Namespace: namespace,
					Labels: map[string]string{
						operatorTag: "true",
					},
				},
				Status: olmv1alpha1.SubscriptionStatus{
					InstalledCSV: csvName,
				},
			}

			subscriptionWithoutLabels = &olmv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      subName,
					Namespace: namespace,
				},
				Status: olmv1alpha1.SubscriptionStatus{
					InstalledCSV: csvName,
				},
			}

			subscriptionDifferentCSV = &olmv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      subName,
					Namespace: namespace,
					Labels: map[string]string{
						operatorTag: "true",
					},
				},
				Status: olmv1alpha1.SubscriptionStatus{
					InstalledCSV: "dummy",
				},
			}

			setup = func(r *ReconcilerTest) error {
				var log = logf.Log.WithName("clusterserviceversion_controller")
				r.Client = fake.NewFakeClient(r.GetGetObjects()...)
				r.Reconciler = &ClusterServiceVersionReconciler{Client: r.Client, Scheme: scheme.Scheme, Log: log}
				return nil
			}

			testClusterServiceVersionWithInstalledCSV = func(t GinkgoTInterface) {
				t.Parallel()
				reconcilerTest := NewReconcilerTest(setup, clusterserviceversion, subscription)
				reconcilerTest.TestAll(t,
					ReconcileStep(opts,
						ReconcileWithExpectedResults(DoneResult)),
					ListStep(opts,
						ListWithObj(&olmv1alpha1.ClusterServiceVersionList{}),
						ListWithFilter(
							client.InNamespace(namespace),
							client.MatchingLabels(map[string]string{
								watchTag: "lite",
							})),
						ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
							list, ok := i.(*olmv1alpha1.ClusterServiceVersionList)

							assert.Truef(t, ok, "expected cluster service version list got type %T", i)
							assert.Equal(t, 1, len(list.Items))
						}),
					),
				)
			}

			testClusterServiceVersionWithoutInstalledCSV = func(t GinkgoTInterface) {
				t.Parallel()
				reconcilerTest := NewReconcilerTest(setup, clusterserviceversion, subscriptionDifferentCSV)
				reconcilerTest.TestAll(t,
					ReconcileStep(nil,
						ReconcileWithExpectedResults(DoneResult)),
					ListStep(nil,
						ListWithObj(&olmv1alpha1.ClusterServiceVersionList{}),
						ListWithFilter(
							client.InNamespace(namespace),
							client.MatchingLabels(map[string]string{
								watchTag: "lite",
							})),
						ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
							list, ok := i.(*olmv1alpha1.ClusterServiceVersionList)

							assert.Truef(t, ok, "expected cluster service version list got type %T", i)
							assert.Equal(t, 0, len(list.Items))
						}),
					),
				)
			}

			testClusterServiceVersionWithSubscriptionWithoutLabels = func(t GinkgoTInterface) {
				t.Parallel()
				reconcilerTest := NewReconcilerTest(setup, clusterserviceversion, subscriptionWithoutLabels)
				reconcilerTest.TestAll(t,
					ReconcileStep(opts,
						ReconcileWithExpectedResults(DoneResult)),
					ListStep(opts,
						ListWithObj(&olmv1alpha1.ClusterServiceVersionList{}),
						ListWithFilter(
							client.InNamespace(namespace),
							client.MatchingLabels(map[string]string{
								watchTag: "lite",
							})),
						ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
							list, ok := i.(*olmv1alpha1.ClusterServiceVersionList)

							assert.Truef(t, ok, "expected cluster service version list got type %T", i)
							assert.Equal(t, 0, len(list.Items))
						}),
					),
				)
			}

			TestBuildMeterDefinitionFromString = func(t GinkgoTInterface) {
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
				err := meter.BuildMeterDefinitionFromString(string(meterStr), csvName, namespace, utils.CSV_ANNOTATION_NAME, utils.CSV_ANNOTATION_NAMESPACE)
				if err != nil {
					t.Errorf("Failed to build MeterDefinition CR: %v", err)
				}
			}
		)

		_ = olmv1alpha1.AddToScheme(scheme.Scheme)
		testClusterServiceVersionWithInstalledCSV(GinkgoT())
		testClusterServiceVersionWithoutInstalledCSV(GinkgoT())
		testClusterServiceVersionWithSubscriptionWithoutLabels(GinkgoT())
		TestBuildMeterDefinitionFromString(GinkgoT())
	})
})
