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
	. "github.com/onsi/ginkgo"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/rectest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("MeterDefinitionController", func() {
	var (
		name      = "meterdefinition"
		namespace = "redhat-marketplace-operator"
		req       = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		opts = []StepOption{
			WithRequest(req),
		}
		meterdefinition = &marketplacev1alpha1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: marketplacev1alpha1.MeterDefinitionSpec{
				Group: "apps.partner.metering.com",
				Kind:  "App",
			},
		}
	)

	var setup = func(r *ReconcilerTest) error {
		log := ctrl.Log.WithName("controllers").WithName("MeterDefinitionController")

		r.Client = fake.NewFakeClientWithScheme(k8sScheme, r.GetGetObjects()...)
		r.Reconciler = &MeterDefinitionReconciler{
			Client:  r.Client,
			Scheme:  k8sScheme,
			Log:     log,
			cc:      reconcileutils.NewClientCommand(r.Client, k8sScheme, log),
			patcher: patch.RHMDefaultPatcher,
		}
		return nil
	}

	var testNoServiceMonitors = func(t GinkgoTInterface) {
		t.Parallel()
		reconcilerTest := NewReconcilerTest(setup, meterdefinition)
		reconcilerTest.TestAll(t,
			ReconcileStep(
				opts,
				ReconcileWithExpectedResults(DoneResult),
			),
		)
	}
	It("meter definition controller", func() {

		testNoServiceMonitors(GinkgoT())
	})
})
