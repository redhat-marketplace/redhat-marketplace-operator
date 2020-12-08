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

package rhm_subscription

import (
	. "github.com/redhat-marketplace/redhat-marketplace-operator/test/rectest"
	"github.com/stretchr/testify/assert"

	. "github.com/onsi/ginkgo"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	opsrcApi "github.com/operator-framework/operator-marketplace/pkg/apis"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Testing with Ginkgo", func() {
	It("RHM subscription controller", func() {

		/*
			defaultFeatures := []string{"meterbase"}
			viper.Set("features", defaultFeatures)
		*/
		_ = opsrcApi.AddToScheme(scheme.Scheme)
		_ = olmv1alpha1.AddToScheme(scheme.Scheme)
		_ = olmv1.AddToScheme(scheme.Scheme)
		testUpdateSubscription(GinkgoT())
	})
})

var (
	name      = rhmOperatorName
	namespace = "openshift-redhat-marketplace"

	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	opts = []StepOption{
		WithRequest(req),
	}

	subscription = &olmv1alpha1.Subscription{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				opreqControlLabel: "true",
			},
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			CatalogSource:          "source",
			CatalogSourceNamespace: "source-namespace",
			Package:                "source-package",
		},
	}
)

func setup(r *ReconcilerTest) error {
	r.Client = fake.NewFakeClient(r.GetGetObjects()...)
	r.Reconciler = &ReconcileSubscription{client: r.Client, scheme: scheme.Scheme}
	return nil
}

func testUpdateSubscription(t GinkgoTInterface) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, subscription)
	reconcilerTest.TestAll(t,
		// Reconcile to create obj
		ReconcileStep(opts,
			ReconcileWithExpectedResults(DoneResult)),
		// List and check results
		ListStep(opts,
			ListWithObj(&olmv1alpha1.SubscriptionList{}),
			ListWithFilter(
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{
					doNotUninstallLabel: "true",
				})),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*olmv1alpha1.SubscriptionList)

				assert.Truef(t, ok, "expected subscription list, got type %T", i)
				assert.Equal(t, 1, len(list.Items))
			}),
		),
	)
}
