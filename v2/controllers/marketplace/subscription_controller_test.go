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

	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/rectest"

	. "github.com/onsi/ginkgo"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	opsrcApi "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Testing with Ginkgo", func() {
	var (
		name             = "new-subscription"
		namespace        = "arbitrary-namespace"
		uninstallSubName = "sub-uninstall"

		req                      reconcile.Request
		opts                     []StepOption
		optsForDeletion          []StepOption
		preExistingOperatorGroup *olmv1.OperatorGroup
		subscription             *olmv1alpha1.Subscription
		subForDeletion           *olmv1alpha1.Subscription
		clusterServiceVersions   *olmv1alpha1.ClusterServiceVersion
	)

	BeforeEach(func() {
		req = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}
		opts = []StepOption{
			WithRequest(req),
		}
		optsForDeletion = []StepOption{
			WithRequest(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      uninstallSubName,
					Namespace: namespace,
				},
			}),
		}

		preExistingOperatorGroup = &olmv1.OperatorGroup{
			ObjectMeta: v1.ObjectMeta{
				Name:      "existing-group",
				Namespace: namespace,
			},
			Spec: olmv1.OperatorGroupSpec{
				TargetNamespaces: []string{
					"arbitrary-namespace",
				},
			},
		}
		subscription = &olmv1alpha1.Subscription{
			ObjectMeta: v1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					operatorTag: "true",
				},
			},
			Spec: &olmv1alpha1.SubscriptionSpec{
				CatalogSource:          "source",
				CatalogSourceNamespace: "source-namespace",
				Package:                "source-package",
			},
		}

		subForDeletion = &olmv1alpha1.Subscription{
			ObjectMeta: v1.ObjectMeta{
				Name:      uninstallSubName,
				Namespace: namespace,
				Labels: map[string]string{
					uninstallTag: "true",
				},
			},
			Spec: &olmv1alpha1.SubscriptionSpec{
				CatalogSource:          "source",
				CatalogSourceNamespace: "source-namespace",
				Package:                "source-package",
			},
			Status: olmv1alpha1.SubscriptionStatus{
				InstalledCSV: "csv-1",
			},
		}

		clusterServiceVersions = &olmv1alpha1.ClusterServiceVersion{
			ObjectMeta: v1.ObjectMeta{
				Name:      "csv-1",
				Namespace: namespace,
			},
		}
	})

	var setup = func(r *ReconcilerTest) error {
		var log = logf.Log.WithName("subscription_controller")
		r.Client = fake.NewFakeClient(r.GetGetObjects()...)
		r.Reconciler = &SubscriptionReconciler{Client: r.Client, Scheme: scheme.Scheme, Log: log}
		return nil
	}

	var testSubscriptionDelete = func(t GinkgoTInterface) {
		t.Parallel()
		reconcilerTest := NewReconcilerTest(setup, subForDeletion, clusterServiceVersions)
		reconcilerTest.TestAll(t,
			ReconcileStep(optsForDeletion,
				ReconcileWithExpectedResults(DoneResult)),
			// List and check results
			ListStep(opts,
				ListWithObj(&olmv1alpha1.SubscriptionList{}),
				ListWithFilter(
					client.InNamespace(namespace)),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
					list, ok := i.(*olmv1alpha1.SubscriptionList)

					assert.Truef(t, ok, "expected subscription list got type %T", i)
					assert.Equal(t, 0, len(list.Items))
				})),
			ListStep(opts,
				ListWithObj(&olmv1alpha1.ClusterServiceVersionList{}),
				ListWithFilter(
					client.InNamespace(namespace)),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
					list, ok := i.(*olmv1alpha1.ClusterServiceVersionList)

					assert.Truef(t, ok, "expected csv list got type %T", i)
					assert.Equal(t, 0, len(list.Items))
				})),
			ListStep(opts,
				ListWithObj(&olmv1.OperatorGroupList{}),
				ListWithFilter(
					client.InNamespace(namespace)),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
					list, ok := i.(*olmv1.OperatorGroupList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 0, len(list.Items))
				})),
		)
	}

	var testNewSubscription = func(t GinkgoTInterface) {
		t.Parallel()
		reconcilerTest := NewReconcilerTest(setup, subscription)
		reconcilerTest.TestAll(t,
			// Reconcile to create obj
			ReconcileStep(opts,
				ReconcileWithExpectedResults(RequeueResult, DoneResult)),
			// List and check results
			ListStep(opts,
				ListWithObj(&olmv1.OperatorGroupList{}),
				ListWithFilter(
					client.InNamespace(namespace),
					client.MatchingLabels(map[string]string{
						operatorTag: "true",
					})),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
					list, ok := i.(*olmv1.OperatorGroupList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 1, len(list.Items))
				}),
			),
		)
	}

	var testNewSubscriptionWithOperatorGroup = func(t GinkgoTInterface) {
		t.Parallel()
		reconcilerTest := NewReconcilerTest(setup, subscription, preExistingOperatorGroup)
		reconcilerTest.TestAll(t,
			ReconcileStep(opts,
				ReconcileWithExpectedResults(DoneResult)),
			ListStep(opts,
				ListWithObj(&olmv1.OperatorGroupList{}),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
					list, ok := i.(*olmv1.OperatorGroupList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 1, len(list.Items))
				}),
			),
		)
	}

	var testDeleteOperatorGroupIfTooMany = func(t GinkgoTInterface) {
		listObjs := []ListStepOption{
			ListWithObj(&olmv1.OperatorGroupList{}),
			ListWithFilter(
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{
					operatorTag: "true",
				})),
		}
		t.Parallel()
		reconcilerTest := NewReconcilerTest(setup, subscription)
		reconcilerTest.TestAll(t,
			// Reconcile to create obj
			ReconcileStep(opts,
				ReconcileWithExpectedResults(RequeueResult, DoneResult)),
			// List and check results
			ListStep(opts,
				append(listObjs,
					ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
						list, ok := i.(*olmv1.OperatorGroupList)

						assert.Truef(t, ok, "expected operator group list got type %T", i)
						assert.Equal(t, 1, len(list.Items))

						r.GetClient().Create(context.TODO(), preExistingOperatorGroup)
					}))...),
			// Reconcile again to delete the extra operator group
			ReconcileStep(opts,
				ReconcileWithExpectedResults(RequeueResult, DoneResult)),
			// Check to make sure we've deleted it
			ListStep(opts,
				append(listObjs,
					ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
						list, ok := i.(*olmv1.OperatorGroupList)

						assert.Truef(t, ok, "expected operator group list got type %T", i)
						assert.Equal(t, 1, len(list.Items))

						r.GetClient().Create(context.TODO(), preExistingOperatorGroup)
					}))...),
		)
	}

	It("subscription controller", func() {
		defaultFeatures := []string{"razee", "meterbase"}
		viper.Set("features", defaultFeatures)
		_ = opsrcApi.AddToScheme(scheme.Scheme)
		_ = olmv1alpha1.AddToScheme(scheme.Scheme)
		_ = olmv1.AddToScheme(scheme.Scheme)
		testNewSubscription(GinkgoT())
		testNewSubscriptionWithOperatorGroup(GinkgoT())
		testDeleteOperatorGroupIfTooMany(GinkgoT())
		testSubscriptionDelete(GinkgoT())
	})
})
