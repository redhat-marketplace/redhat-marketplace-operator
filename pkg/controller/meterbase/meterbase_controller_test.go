package meterbase

import (
	"context"
	"testing"

	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	 corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
func TestMeterBaseController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	var (
		name      = "marketplace-operator"
		namespace = "rhm-marketplace"
	)

	// A Memcached resource with metadata and spec.
	meterbase := &marketplacev1alpha1.MeterBase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MeterBaseSpec{
			Enabled: true,
			Prometheus: &marketplacev1alpha1.PrometheusSpec{
				Storage: marketplacev1alpha1.StorageSpec{
					Size: resource.MustParse("30Gi"),
				},
			},
		},
	}
	// Objects to track in the fake client.
	objs := []runtime.Object{
		meterbase,
	}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterbase)
	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	// Create a ReconcileMeterBase object with the scheme and fake client.
	r := &ReconcileMeterBase{client: cl, scheme: s}

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

	configMap := &corev1.ConfigMap{}

	// Check if configmap has been created
	err = cl.Get(context.TODO(), req.NamespacedName, configMap)
	if err != nil {
		t.Fatalf("get configmap: (%v)", err)
	}

	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Check the result of reconciliation to make sure it has the desired state.
	if res.Requeue {
		t.Error("reconcile requeue which is not expected")
	}

	// Check if Deployment has been created and has the correct size.
	dep := &appsv1.StatefulSet{}
	err = cl.Get(context.TODO(), req.NamespacedName, dep)
	if err != nil {
		t.Fatalf("get statefulset: (%v)", err)
	}

	if len(dep.Spec.VolumeClaimTemplates) != 1 {
		t.Errorf("volume claim count (%d) is not the expected size (%d)", len(dep.Spec.VolumeClaimTemplates), 1)
	}

	vctSpec := dep.Spec.VolumeClaimTemplates[0].Spec
	size := vctSpec.Resources.Requests.StorageEphemeral()
	expectedSize := resource.MustParse("30Gi")
	if !expectedSize.Equal(*size) {
		t.Errorf("volume claim (%v) is not the expected size (%v)", size, expectedSize)
	}

	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Check the result of reconciliation to make sure it has the desired state.
	if res.Requeue {
		t.Error("reconcile requeue which is not expected")
	}

	// Check if Service has been created.
	// ser := &corev1.Service{}
	// err = cl.Get(context.TODO(), req.NamespacedName, ser)
	// if err != nil {
	// 	t.Fatalf("get service: (%v)", err)
	// }

	// Create the 3 expected pods in namespace and collect their names to check
	// later.
	// podLabels := labelsForMemcached(name)
	// pod := corev1.Pod{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Namespace: namespace,
	// 		Labels:    podLabels,
	// 	},
	// }
	// podNames := make([]string, 3)
	// for i := 0; i < 3; i++ {
	// 	pod.ObjectMeta.Name = name + ".pod." + strconv.Itoa(rand.Int())
	// 	podNames[i] = pod.ObjectMeta.Name
	// 	if err = cl.Create(context.TODO(), pod.DeepCopy()); err != nil {
	// 		t.Fatalf("create pod %d: (%v)", i, err)
	// 	}
	// }

	// Reconcile again so Reconcile() checks pods and updates the Memcached
	// resources' Status.
	// res, err = r.Reconcile(req)
	// if err != nil {
	// 	t.Fatalf("reconcile: (%v)", err)
	// }
	// if res != (reconcile.Result{}) {
	// 	t.Error("reconcile did not return an empty Result")
	// }

	// Get the updated Memcached object.
	// memcached = &cachev1alpha1.Memcached{}
	// err = r.client.Get(context.TODO(), req.NamespacedName, memcached)
	// if err != nil {
	// 	t.Errorf("get memcached: (%v)", err)
	// }

	// Ensure Reconcile() updated the Memcached's Status as expected.
	// nodes := memcached.Status.Nodes
	// if !reflect.DeepEqual(podNames, nodes) {
	// 	t.Errorf("pod names %v did not match expected %v", nodes, podNames)
	// }
}
