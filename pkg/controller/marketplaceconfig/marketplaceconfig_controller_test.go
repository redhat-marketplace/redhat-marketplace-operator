package marketplaceconfig

import (
	"testing"

	. "github.ibm.com/symposium/marketplace-operator/test/controller"

	opsrcApi "github.com/operator-framework/operator-marketplace/pkg/apis"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/marketplace-operator/pkg/utils"
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

	t.Run("Test Clean Install", testCleanInstall)
}

var (
	name                 = "markeplaceconfig"
	namespace            = "redhat-marketplace-operator"
	customerID    string = "example-userid"
	razeeName            = "rhm-marketplaceconfig-razeedeployment"
	meterBaseName        = "rhm-marketplaceconfig-meterbase"
	req                  = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	opts = []ReconcilerTestCaseOption{
		WithRequest(req),
		WithNamespacedName(types.NamespacedName{Namespace: namespace, Name: name}),
  }

	marketplaceconfig = buildMarketplaceConfigCR(name, namespace, customerID)
	razeedeployment   = utils.BuildRazeeCr(namespace, marketplaceconfig.Spec.ClusterUUID, marketplaceconfig.Spec.DeploySecretName)
	meterbase         = utils.BuildMeterBaseCr(namespace)
)

func setup(objs ...runtime.Object) ReconcilerSetupFunc {
	return func(r *ReconcilerTest) error {
		s := scheme.Scheme
		_ = opsrcApi.AddToScheme(s)
		s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, marketplaceconfig)
		s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, razeedeployment)
		s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterbase)

		r.Client = fake.NewFakeClient(objs...)
		r.Reconciler = &ReconcileMarketplaceConfig{client: r.Client, scheme: s}
		return nil
	}
}

func testCleanInstall(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup(marketplaceconfig.DeepCopy()))
	reconcilerTest.TestAll(t,
		[]*ReconcilerTestCase{
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&marketplacev1alpha1.RazeeDeployment{}),
					WithName(razeeName))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&marketplacev1alpha1.MeterBase{}),
					WithName(meterBaseName))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&opsrcv1.OperatorSource{}),
					WithNamespacedName(types.NamespacedName{Namespace: utils.OPERATOR_MKTPLACE_NS, Name: utils.OPSRC_NAME}))...),
		})
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
			RhmAccountID: customerID,
		},
	}
}
