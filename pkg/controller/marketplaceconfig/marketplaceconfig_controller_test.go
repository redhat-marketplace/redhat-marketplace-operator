package marketplaceconfig

import (
	"context"
	"testing"

	opsrcApi "github.com/operator-framework/operator-marketplace/pkg/apis"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/marketplace-operator/pkg/utils"
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

	defaultFeatures := []string{"razee", "meterbase"}
	viper.Set("assets", "../../../assets")
	viper.Set("features", defaultFeatures)

	var (
		name                 = "markeplaceconfig"
		opsrcName            = "redhat-marketplace-operators"
		razeeName            = "rhm-marketplaceconfig-razeedeployment"
		meterBaseName        = "rhm-marketplaceconfig-meterbase"
		namespace            = "redhat-marketplace-operator"
		customerID    string = "example-userid"
	)

	// Declare resources
	marketplaceconfig := buildMarketplaceConfigCR(name, namespace, customerID)
	opsrc := utils.BuildNewOpSrc()
	razeedeployment := utils.BuildRazeeCr(namespace, marketplaceconfig.Spec.ClusterUUID, marketplaceconfig.Spec.DeploySecretName)
	meterbase := utils.BuildMeterBaseCr(namespace)

	// Objects to track in the fake client.
	objs := []runtime.Object{
		marketplaceconfig,
	}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add schemes
	if err := opsrcApi.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add OperatorSource Scheme: (%v)", err)
	}
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, marketplaceconfig)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, razeedeployment)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterbase)
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

	// Reconcile again so Reconcile() checks for RazeeDeployment
	req.Name = name
	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	if res != (reconcile.Result{Requeue: true}) {
		t.Error("reconcile did not requeue as expected")
	}
	// Get the updated RazeeDeployment Object
	req.Name = razeeName
	razeedeployment = &marketplacev1alpha1.RazeeDeployment{}
	err = r.client.Get(context.TODO(), req.NamespacedName, razeedeployment)
	if err != nil {
		t.Errorf("get RazeeDeployment: (%v)", err)
	}

	// Reconcile again so Reconcile() checks for MeterBase
	req.Name = name
	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	if res != (reconcile.Result{Requeue: true}) {
		t.Error("reconcile did not requeue as expected")
	}

	// Get the updated MeterBase Object
	req.Name = meterBaseName
	meterbase = &marketplacev1alpha1.MeterBase{}
	err = r.client.Get(context.TODO(), req.NamespacedName, meterbase)
	if err != nil {
		t.Errorf("get meterbase: (%v)", err)
	}

	//Reconcile again so Reconcile() checks for the OperatorSource
	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	if res != (reconcile.Result{}) {
		t.Error("reconcile did not result with expected outcome of Requeue: False")
	}
	// Get the updated OperatorSource object
	req.Name = opsrcName
	opsrc = &opsrcv1.OperatorSource{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: utils.OPERATOR_MKTPLACE_NS,
	}, opsrc)
	if err != nil {
		t.Errorf("get OperatorSource: (%v)", err)
	}

}

// Test whether flags have been set or not
func TestMarketplaceConfigControllerFlags(t *testing.T) {
	flagset := FlagSet()

	if !flagset.HasFlags() {
		t.Errorf("no flags on flagset")
	}
}

func buildMarketplaceConfigCR(name, namespace, customerID string) *marketplacev1alpha1.MarketplaceConfig {
	return &marketplacev1alpha1.MarketplaceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MarketplaceConfigSpec{
			RhmCustomerID: customerID,
		},
	}
}
