package e2e

import (
	"testing"
	"time"

	"github.ibm.com/symposium/marketplace-operator/pkg/apis"
	operator "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

func TestMarketplaceConfig(t *testing.T) {
	marketplaceConfigList := &operator.MarketplaceConfigList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, marketplaceConfigList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
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
	// wait for marketplace-operator to be ready
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "marketplace-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}
}
