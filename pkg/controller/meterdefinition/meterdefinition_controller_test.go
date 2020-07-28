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

package meterdefinition

import (
	"testing"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/logger"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/test/rectest"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestMeterDefinitionController(t *testing.T) {
	logger.SetLoggerToDevelopmentZap()
	viper.Set("assets", "../../../assets")

	t.Run("Test No Service Monitors", testNoServiceMonitors)
}

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
			Group:   "apps.partner.metering.com",
			Kind:    "App",
			Version: "v1",
			ServiceMeters: []string{
				"rpc_duration_seconds.*",
			},
			PodMeters: []string{
				"foo",
			},
		},
	}
)

func setup(r *ReconcilerTest) error {
	s := scheme.Scheme
	_ = monitoringv1.AddToScheme(s)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterdefinition)

	r.Client = fake.NewFakeClient(r.GetGetObjects()...)
	r.Reconciler = &ReconcileMeterDefinition{client: r.Client, scheme: s}
	return nil
}

func testNoServiceMonitors(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, meterdefinition)
	reconcilerTest.TestAll(t,
		ReconcileStep(
			opts,
			ReconcileWithExpectedResults(DoneResult),
		),
	)
}
