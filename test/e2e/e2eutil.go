// +build e2e

package e2e

import (
	"fmt"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	e2eutils "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"testing"
	"time"
)

func waitForStatefulSet(t *testing.T, kubeclient kubernetes.Interface, namespace, name string, replicas int,
	retryInterval, timeout time.Duration) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		statefulset, err := kubeclient.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s statefulset\n", name)
				return false, nil
			}
			return false, err
		}

		if int(statefulset.Status.ReadyReplicas) >= replicas {
			return true, nil
		}
		t.Logf("Waiting for full availability of %s statefulset (%d/%d)\n", name,
			statefulset.Status.ReadyReplicas, replicas)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("StatefulSet available (%d/%d)\n", replicas, replicas)
	return nil
}

func waitForBatchJob(t *testing.T, kubeclient kubernetes.Interface, namespace, name string,
	retryInterval, timeout time.Duration) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		razeejob, err := kubeclient.BatchV1().Jobs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s job\n", razeejob.Name)
				return false, nil
			}
			return false, err
		}
		// job is available
		return true, nil
	})
	if err != nil {
		return err
	}
	t.Logf("Job is available")
	return nil
}

// Test that a certain CR is deployed - with the passed name
func crDeployedTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, name string, replicas int) error {
	namespace, err := ctx.GetNamespace()

	err = e2eutils.WaitForDeployment(t, f.KubeClient, namespace, name, replicas, time.Second*5, time.Second*30)
	if err != nil {
		return fmt.Errorf("Failed waiting for deployment %v", err)
	}
	return nil
}
