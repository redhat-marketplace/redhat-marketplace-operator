package meterdefinition

import (
	"testing"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	. "github.ibm.com/symposium/redhat-marketplace-operator/test/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"k8s.io/client-go/kubernetes/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestMeterDefinitionController(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

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

	opts = []TestCaseOption{
		WithRequest(req),
		WithNamespace(namespace),
		WithName(name),
	}
	meterdefinition = &marketplacev1alpha1.MeterDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MeterDefinitionSpec{
			MeterDomain:  "apps.partner.metering.com",
			MeterKind:    "App",
			MeterVersion: "v1",
			ServiceMeterLabels: []string{
				"rpc_duration_seconds.*",
			},
			PodMeterLabels: []string{
				"foo",
			},
			ServiceMonitorNamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                            "example-app",
					"marketplace.redhat.com/metered": "true",
				},
			},
		},
	}
)

func setup(r *ReconcilerTest) error {
	s := scheme.Scheme
	_ = monitoringv1.AddToScheme(s)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterdefinition)

	r.Client = fake.NewFakeClient(r.GetRuntimeObjects()...)
	r.Reconciler = &ReconcileMeterDefinition{client: r.Client, scheme: s}
	return nil
}

func testNoServiceMonitors(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, meterdefinition)
	reconcilerTest.TestAll(t,
		[]TestCaseStep{
			NewReconcileStep(
				append(opts,
					WithExpectedResult(reconcile.Result{}),
				)...,
			),
		})
}
