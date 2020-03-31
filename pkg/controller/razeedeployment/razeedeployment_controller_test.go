package razeedeployment

import (
	"context"
	"fmt"
	"testing"

	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// TestMeterBaseController runs ReconcileMemcached.Reconcile() against a
// fake client that tracks a MeterBase object.
func TestRazeeDeployController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	viper.Set("assets", "../../../assets")

	var (
		name      = "marketplaceconfig"
		namespace = "redhat-marketplace-operator"
	)

	// A resource with metadata and spec.
	razeeDeployment := &marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled: true,
			ClusterUUID: "foo"
		},
	}
	// Objects to track in the fake client.
	objs := []runtime.Object{
		razeeDeployment,
	}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, razeeDeployment)
	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	// Create a ReconcileMeterBase object with the scheme and fake client.
	r := &ReconcileRazeeDeployment{client: cl, scheme: s}
	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: "redhat-marketplace-operator",
		},
	}
	_, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// check that missing resources are being picked up
	razeeDeployment = &marketplacev1alpha1.RazeeDeployment{}
	// Check if razeedeployJob has been created
	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "razeedeploy-job",
			Namespace: "redhat-marketplace-operator",
		},
	}

	err = cl.Get(context.TODO(), req.NamespacedName, razeeDeployment)
	if err != nil {
		t.Fatalf("get razeedeployment: (%v)", err)
	}

	if len(*razeeDeployment.Status.MissingValuesFromSecret) > 0 {
		fmt.Println("missing resources found", razeeDeployment.Status.MissingValuesFromSecret)
	}

	// Check the result of reconciliation to make sure it has the desired state.
	// if res.Requeue {
	// 	t.Error("reconcile requeue which is not expected")
	// }

}

func CreateWatchKeeperSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "razeedeploy-job",
			Namespace: "marketplace-operator",
		},
	}
}
