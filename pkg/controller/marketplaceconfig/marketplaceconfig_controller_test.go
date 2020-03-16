package marketplaceconfig

import (
	"context"
	"testing"

	opsrcApi "github.com/operator-framework/operator-marketplace/pkg/apis"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestMarketplaceConfigController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	viper.Set("assets", "../../../assets")

	var (
		opsrcName       = "redhat-marketplace-operators"
		name            = "markeplaceconfig"
		namespace       = "redhat-marketplace-operator"
		replicas  int32 = 1
	)

	// A MarketplaceConfig resource with metadata and spec.
	marketplaceconfig := &marketplacev1alpha1.MarketplaceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MarketplaceConfigSpec{
			Size: replicas,
		},
	}

	// An OperatorSource resource with metadata and spec
	opsrc := &opsrcv1.OperatorSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opsrcName,
			Namespace: namespace,
		},
		Spec: opsrcv1.OperatorSourceSpec{
			DisplayName:       "Red Hat Marketplace",
			Endpoint:          "https://quay.io/cnr",
			Publisher:         "Red Hat Marketplace",
			RegistryNamespace: "redhat-marketplace",
			Type:              "appregistry",
		},
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{
		marketplaceconfig,
	}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Third party scheme (operator-source) have to add manually.
	if err := opsrcApi.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add OperatorSource Scheme: (%v)", err)
	}

	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, marketplaceconfig)
	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	// Create a ReconcileMeterBase object with the scheme and fake client.
	r := &ReconcileMarketplaceConfig{client: cl, scheme: s}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Check the result of reconciliation to make sure it has the desired state.
	if !res.Requeue {
		t.Error("reconcile did not requeue request as expected")
	}

	// Check if Deployment has been created and has the correct size.
	dep := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), req.NamespacedName, dep)
	if err != nil {
		t.Fatalf("get deployment: (%v)", err)
	}

	// Check the result of reconciliation to make sure it has the desired state.
	if !res.Requeue {
		t.Error("reconcile did not requeue request as expected")
	}

	//Reconcile again so Reconcile() checks pods and updates the MarketplaceConfig
	//resources' Status.
	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	if res != (reconcile.Result{Requeue: true}) {
		t.Error("reconcile did not requeue request as expected")
	}

	// Get the updated MarketplaceConfig object.
	marketplaceconfig = &marketplacev1alpha1.MarketplaceConfig{}
	err = r.client.Get(context.TODO(), req.NamespacedName, marketplaceconfig)
	if err != nil {
		t.Errorf("get marketplaceConfig: (%v)", err)
	}

	//Reconcile again so Reconcile() checks OperatorSource
	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	if res != (reconcile.Result{}) {
		t.Error("reconcile did not return the expected result, requeue=false")
	}

	req.Name = opsrcName

	opsrc = &opsrcv1.OperatorSource{}
	err = r.client.Get(context.TODO(), req.NamespacedName, opsrc)
	if err != nil {
		t.Errorf("get OperatorSource: (%v)", err)
	}

}

// Test whether flags have been set or not
func TestMeterBaseControllerFlags(t *testing.T) {
	flagset := FlagSet()

	if !flagset.HasFlags() {
		t.Errorf("no flags on flagset")
	}
}
