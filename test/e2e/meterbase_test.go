// +build e2e

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

func TestMeterbase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	meterbaseConfigList := &operator.MeterBaseList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, meterbaseConfigList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	// run subtests
	t.Run("meterbase-group", func(t *testing.T) {
		t.Run("Cluster", MeterbaseOperatorCluster)
		t.Run("Cluster2", MeterbaseOperatorCluster)
	})
}

// TODO: add tests for meterbase
// func memcachedScaleTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
// 	namespace, err := ctx.GetNamespace()
// 	if err != nil {
// 		return fmt.Errorf("could not get namespace: %v", err)
// 	}
// 	// create memcached custom resource
// 	exampleMeterBase := &operator.MeterBase{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      "example-memcached",
// 			Namespace: namespace,
// 		},
// 		Spec: operator.MeterBaseSpec{
// 			Size: 3,
// 		},
// 	}
// 	// use TestCtx's create helper to create the object and add a cleanup function for the new object
// 	err = f.Client.Create(goctx.TODO(), exampleMeterBase, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
// 	if err != nil {
// 		return err
// 	}
// 	// wait for example-memcached to reach 3 replicas
// 	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "example-memcached", 3, retryInterval, timeout)
// 	if err != nil {
// 		return err
// 	}

// 	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "example-memcached", Namespace: namespace}, exampleMeterBase)
// 	if err != nil {
// 		return err
// 	}
// 	exampleMeterBase.Spec.Size = 4
// 	err = f.Client.Update(goctx.TODO(), exampleMeterBase)
// 	if err != nil {
// 		return err
// 	}

// 	// wait for example-memcached to reach 4 replicas
// 	return e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "example-memcached", 4, retryInterval, timeout)
//}

func MeterbaseOperatorCluster(t *testing.T) {
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
	// wait for meterbase-operator to be ready
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "marketplace-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	// if err = memcachedScaleTest(t, f, ctx); err != nil {
	// 	t.Fatal(err)
	// }
}
