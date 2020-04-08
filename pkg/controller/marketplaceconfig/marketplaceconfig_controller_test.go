package marketplaceconfig

import (
	"context"
	"path/filepath"
	gruntime "runtime"
	"testing"

	opsrcApi "github.com/operator-framework/operator-marketplace/pkg/apis"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/marketplace-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
		razeeName            = "rhm-marketplaceconfig-razeedeployment"
		meterBaseName        = "rhm-marketplaceconfig-meterbase"
		namespace            = "redhat-marketplace-operator"
		customerID    string = "example-userid"
	)

	// Declare resources
	marketplaceconfig := buildMarketplaceConfigCR(name, namespace, customerID)
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

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	r.reconcileTest(t,
		req,
		name,
		namespace,
		&appsv1.Deployment{},
		reconcile.Result{Requeue: true})

	r.reconcileTest(t,
		req,
		razeeName,
		namespace,
		&marketplacev1alpha1.RazeeDeployment{},
		reconcile.Result{Requeue: true})

	r.reconcileTest(t,
		req,
		meterBaseName,
		namespace,
		&marketplacev1alpha1.MeterBase{},
		reconcile.Result{Requeue: true})

	r.reconcileTest(t,
		req,
		utils.CLUSTER_ROLE,
		namespace,
		&corev1.ServiceAccount{},
		reconcile.Result{Requeue: true})

	r.reconcileTest(t,
		req,
		utils.CLUSTER_ROLE_BINDING,
		"",
		&rbacv1.ClusterRoleBinding{},
		reconcile.Result{Requeue: true})

	r.reconcileTest(t,
		req,
		utils.OPSRC_NAME,
		utils.OPERATOR_MKTPLACE_NS,
		&opsrcv1.OperatorSource{},
		reconcile.Result{Requeue: true})
}

// Test whether flags have been set or not
func TestMarketplaceConfigControllerFlags(t *testing.T) {
	flagset := FlagSet()

	if !flagset.HasFlags() {
		t.Errorf("no flags on flagset")
	}
}

func (r *ReconcileMarketplaceConfig) reconcileTest(
	t *testing.T,
	req reconcile.Request,
	name, namespace string,
	obj runtime.Object,
	expectedResult reconcile.Result) {

	_, fn, line, _ := gruntime.Caller(1)
	//Reconcile again so Reconcile() checks for the OperatorSource
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("[%s:%d] reconcile: (%v)", filepath.Base(fn), line, err)
	}
	if res != (expectedResult) {
		t.Errorf("[%s:%d] %v reconcile result(%v) != expected(%v)", filepath.Base(fn), line, req, res, expectedResult)
	}

	err = r.client.Get(context.TODO(), types.NamespacedName{namespace, name}, obj)

	if err != nil {
		t.Errorf("[%s:%d] get (%T): (%v)", filepath.Base(fn), line, obj, err)
	}
}

func buildMarketplaceConfigCR(name, namespace, customerID string) *marketplacev1alpha1.MarketplaceConfig {
	return &marketplacev1alpha1.MarketplaceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MarketplaceConfigSpec{
			RhmAccountID: customerID,
		},
	}
}
