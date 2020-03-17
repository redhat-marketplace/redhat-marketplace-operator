// +build e2e

package e2e

import (
	goctx "context"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.ibm.com/symposium/marketplace-operator/pkg/apis"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	marketplaceConfigList := &marketplacev1alpha1.MarketplaceConfigList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, marketplaceConfigList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	viper.Set("autoinstall", true)
	// run subtests
	t.Run("marketplaceconfig-group", func(t *testing.T) {
		t.Run("Cluster3", MarketplaceOperatorCluster)
		// t.Run("Cluster4", MarketplaceOperatorCluster)
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

	exampleMarketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-marketplaceconfig",
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MarketplaceConfigSpec{
			Size: 1,
		},
	}

	err = f.Client.Create(goctx.TODO(), exampleMarketplaceConfig, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		t.Fatal(err)
	}

	if err = crDeployedTest(t, f, ctx, "example-marketplaceconfig", 1); err != nil {
		t.Error(err)
	}
	if err = crDeployedTest(t, f, ctx, "marketplaceconfig-razeedeployment", 1); err != nil {
		t.Error(err)
	}
	if err = crDeployedTest(t, f, ctx, "marketplaceconfig-meterbase", 1); err != nil {
		t.Error(err)
	}
}

// Test that when MarketplaceConfig CR is deployed, so is a RazeePod
func crDeployedTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, name string, replicas int) error {
	namespace, err := ctx.GetNamespace()

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, name, replicas, time.Second*5, time.Second*30)
	if err != nil {
		return fmt.Errorf("Failed waiting for deployment %v", err)
	}
	return nil
}
