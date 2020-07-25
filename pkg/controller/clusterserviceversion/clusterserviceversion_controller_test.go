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

package clusterserviceversion

import (
	"testing"

	. "github.com/redhat-marketplace/redhat-marketplace-operator/test/rectest"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestClusterServiceVersionController(t *testing.T) {

	logf.SetLogger(logf.ZapLogger(true))
	_ = olmv1alpha1.AddToScheme(scheme.Scheme)

	t.Run("Test New Cluster Service Version with installed CSV", testClusterServiceVersionWithInstalledCSV)
	t.Run("Test New Cluster Service Version without installed CSV", testClusterServiceVersionWithoutInstalledCSV)
	t.Run("Test New Cluster Service Version with Sub without labels", testClusterServiceVersionWithSubscriptionWithoutLabels)
}

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
		ObjectMeta: v1.ObjectMeta{
			Name:      csvName,
			Namespace: namespace,
		},
	}
	subscription = &olmv1alpha1.Subscription{
		ObjectMeta: v1.ObjectMeta{
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
		ObjectMeta: v1.ObjectMeta{
			Name:      subName,
			Namespace: namespace,
		},
		Status: olmv1alpha1.SubscriptionStatus{
			InstalledCSV: csvName,
		},
	}

	subscriptionDifferentCSV = &olmv1alpha1.Subscription{
		ObjectMeta: v1.ObjectMeta{
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
)

func setup(r *ReconcilerTest) error {
	r.Client = fake.NewFakeClient(r.GetGetObjects()...)
	r.Reconciler = &ReconcileClusterServiceVersion{client: r.Client, scheme: scheme.Scheme}
	return nil
}

func testClusterServiceVersionWithInstalledCSV(t *testing.T) {
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

func testClusterServiceVersionWithoutInstalledCSV(t *testing.T) {
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

func testClusterServiceVersionWithSubscriptionWithoutLabels(t *testing.T) {
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
