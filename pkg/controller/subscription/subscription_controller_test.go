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

package subscription

import (
	"testing"

	"context"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/test/rectest"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	opsrcApi "github.com/operator-framework/operator-marketplace/pkg/apis"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestSubscriptionController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	defaultFeatures := []string{"razee", "meterbase"}
	viper.Set("assets", "../../../assets")
	viper.Set("features", defaultFeatures)
	_ = opsrcApi.AddToScheme(scheme.Scheme)
	_ = olmv1alpha1.AddToScheme(scheme.Scheme)
	_ = olmv1.AddToScheme(scheme.Scheme)

	t.Run("Test New Subscription", testNewSubscription)
	t.Run("Test New Sub with Existing OG", testNewSubscriptionWithOperatorGroup)
	t.Run("Test Sub with OG Added", testDeleteOperatorGroupIfTooMany)
	t.Run("Test Sub deleted", testNewSubscriptionDelete)
}

var (
	name      = "new-subscription"
	namespace = "arbitrary-namespace"

	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	opts = []StepOption{
		WithRequest(req),
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

	installPlan = &olmv1alpha1.InstallPlan{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: namespace,
		},
		Spec:   olmv1alpha1.InstallPlanSpec{},
		Status: olmv1alpha1.InstallPlanStatus{},
	}

	installPlanRef = &olmv1alpha1.InstallPlanReference{
		Name: "test",
	}

	currentCSV = &olmv1alpha1.ClusterServiceVersion{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: namespace,
		},
		Spec:   olmv1alpha1.ClusterServiceVersionSpec{},
		Status: olmv1alpha1.ClusterServiceVersionStatus{},
	}

	objRef = &corev1.ObjectReference{
		Name:      "test",
		Namespace: namespace,
	}

	subscriptionWithStatus = &olmv1alpha1.Subscription{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				operatorTag: "true",
				uninstallTag: "true",
			},
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			CatalogSource:          "source",
			CatalogSourceNamespace: "source-namespace",
			Package:                "source-package",
		},
		Status: olmv1alpha1.SubscriptionStatus{
			CurrentCSV:     "test",
			InstallPlanRef: objRef,
		},
	}
)

func setup(r *ReconcilerTest) error {
	r.Client = fake.NewFakeClient(r.GetGetObjects()...)
	r.Reconciler = &ReconcileSubscription{client: r.Client, scheme: scheme.Scheme}
	return nil
}

func testNewSubscriptionDelete(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, subscriptionWithStatus, currentCSV, preExistingOperatorGroup)
	reconcilerTest.TestAll(t,
		ReconcileStep(opts,
			ReconcileWithExpectedResults(DoneResult)),
		// List and check results
		ListStep(opts,
			ListWithObj(&olmv1alpha1.SubscriptionList{}),
			ListWithFilter(
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{
					operatorTag: "true",
					uninstallTag: "true",
				}),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
				list, ok := i.(*olmv1alpha1.SubscriptionList)

				assert.Truef(t, ok, "expected subscription list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			})),
		ListStep(opts,
			ListWithObj(&olmv1alpha1.ClusterServiceVersionList{}),
			ListWithFilter(
				client.InNamespace(namespace)),
			ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
				list, ok := i.(*olmv1alpha1.ClusterServiceVersionList)

				assert.Truef(t, ok, "expected csv list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			})),
	)
}

func testNewSubscription(t *testing.T) {
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
			ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
				list, ok := i.(*olmv1.OperatorGroupList)

				assert.Truef(t, ok, "expected operator group list got type %T", i)
				assert.Equal(t, 1, len(list.Items))
			}),
		),
	)
}

func testNewSubscriptionWithOperatorGroup(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, subscription, preExistingOperatorGroup)
	reconcilerTest.TestAll(t,
		ReconcileStep(opts,
			ReconcileWithExpectedResults(DoneResult)),
		ListStep(opts,
			ListWithObj(&olmv1.OperatorGroupList{}),
			ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
				list, ok := i.(*olmv1.OperatorGroupList)

				assert.Truef(t, ok, "expected operator group list got type %T", i)
				assert.Equal(t, 1, len(list.Items))
			}),
		),
	)
}

func testDeleteOperatorGroupIfTooMany(t *testing.T) {
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
				ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
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
				ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
					list, ok := i.(*olmv1.OperatorGroupList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 1, len(list.Items))

					r.GetClient().Create(context.TODO(), preExistingOperatorGroup)
				}))...),
	)
}
