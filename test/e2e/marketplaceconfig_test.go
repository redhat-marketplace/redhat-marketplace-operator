// +build e2e

package e2e

import (
	goctx "context"
	"testing"
	"time"

	opsrcHelper "github.com/operator-framework/operator-marketplace/test/helpers"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/spf13/viper"
	"github.ibm.com/symposium/marketplace-operator/pkg/apis"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/marketplace-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

func TestMarketplaceConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	marketplaceConfigList := &marketplacev1alpha1.MarketplaceConfigList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, marketplaceConfigList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	meterbaseConfigList := &marketplacev1alpha1.MeterBaseList{}
	err = framework.AddToFrameworkScheme(apis.AddToScheme, meterbaseConfigList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	razeeDeploymentList := &marketplacev1alpha1.RazeeDeploymentList{}
	err = framework.AddToFrameworkScheme(apis.AddToScheme, razeeDeploymentList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	viper.Set("assets", "../../../assets")
	defaultFeatures := []string{"razee", "meterbase"}
	viper.Set("features", defaultFeatures)

	// run subtests
	t.Run("marketplaceconfig-group", func(t *testing.T) {
		t.Run("Cluster3", MarketplaceOperatorCluster)
		t.Run("Cluster4", MarketplaceOperatorCluster)
	})
}

func MarketplaceOperatorCluster(t *testing.T) {
	t.Parallel()
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{
		TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}
	// get global framework variables
	f := framework.Global
	// wait for redhat-marketplace-operator to be ready
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "redhat-marketplace-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	exampleMarketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.MARKETPLACECONFIG_NAME,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MarketplaceConfigSpec{
			RhmAccountID: "example-userid",
		},
	}

	err = f.Client.Create(goctx.TODO(), exampleMarketplaceConfig, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		t.Fatal(err)
	}
	// Checks if an OperatorSoure object has been deployed
	if opsrcHelper.WaitForOpsrcMarkedForDeletionWithFinalizer(f.Client, utils.OPSRC_NAME, utils.OPERATOR_MKTPLACE_NS); err != nil {
		t.Error(err)
	}
	// Checks if a RazeeJob has been deployed
	if waitForBatchJob(t, f.KubeClient, namespace, utils.RAZEE_JOB_NAME, retryInterval, timeout); err != nil {
		t.Error(err)
	}
	// Checks if a statefulset for MeterBase has been deployed
	err = waitForStatefulSet(t, f.KubeClient, namespace, utils.METERBASE_NAME, 1, retryInterval, timeout)
	if err != nil {
		t.Error(err)
	}
}
