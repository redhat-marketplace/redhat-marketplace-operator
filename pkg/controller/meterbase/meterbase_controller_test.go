// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package meterbase

import (
	// "context"
	// "math/rand"
	// "strconv"
	"testing"

	"github.com/spf13/viper"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/test/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	name      = "redhat-marketplace-operator"
	namespace = "rhm-marketplace"
	meterbase = &marketplacev1alpha1.MeterBase{
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
	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	options = []TestCaseOption{
		WithRequest(req),
		WithNamespace(namespace),
	}
)

// TestMeterBaseController runs ReconcileMeterBase.Reconcile() against a
// fake client that tracks a MeterBase object.
func TestMeterBaseController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	viper.Set("assets", "../../../assets")

}

func setup(r *ReconcilerTest) error {
	s := scheme.Scheme
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterbase)
	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(r.GetRuntimeObjects()...)
	// Create a ReconcileMeterBase object with the scheme and fake client.
	rm := &ReconcileMeterBase{client: cl, scheme: s}

	r.SetClient(cl)
	r.SetReconciler(rm)
	return nil
}

func testCleanInstall(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, meterbase)
	reconcilerTest.TestAll(t, []TestCaseStep{
		NewReconcileStep(options...),
		NewReconcilerTestCase(append(options,
			WithTestObj(&corev1.ConfigMap{}),
			WithName(name))...),
	})
	// Register operator types with the runtime scheme.

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .

	// res, err = r.Reconcile(req)
	// if err != nil {
	// 	t.Fatalf("reconcile: (%v)", err)
	// }
	// // Check the result of reconciliation to make sure it has the desired state.
	// if !res.Requeue {
	// 	t.Error("reconcile did not requeue request as expected")
	// }

	// // Check if Deployment has been created and has the correct size.
	// dep := &appsv1.StatefulSet{}
	// err = cl.Get(context.TODO(), req.NamespacedName, dep)
	// if err != nil {
	// 	t.Fatalf("get statefulset: (%v)", err)
	// }

	// if len(dep.Spec.VolumeClaimTemplates) != 1 {
	// 	t.Errorf("volume claim count (%d) is not the expected size (%d)", len(dep.Spec.VolumeClaimTemplates), 1)
	// }

	// vctSpec := dep.Spec.VolumeClaimTemplates[0].Spec
	// size := vctSpec.Resources.Requests["storage"]
	// expectedSize := resource.MustParse("30Gi")

	// if !expectedSize.Equal(size) {
	// 	t.Errorf("volume claim (%v) is not the expected size (%v)", size, expectedSize)
	// }

	// res, err = r.Reconcile(req)
	// if err != nil {
	// 	t.Fatalf("reconcile: (%v)", err)
	// }

	// // Check the result of reconciliation to make sure it has the desired state.
	// if !res.Requeue {
	// 	t.Error("reconcile did not requeue request as expected")
	// }

	// //Check if Service has been created.
	// ser := &corev1.Service{}
	// err = cl.Get(context.TODO(), req.NamespacedName, ser)
	// if err != nil {
	// 	t.Fatalf("get service: (%v)", err)
	// }

	// // Create the 3 expected pods in namespace and collect their names to check
	// // later.
	// podLabels := labelsForPrometheus(name)
	// pod := corev1.Pod{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Namespace: namespace,
	// 		Labels:    podLabels,
	// 	},
	// }
	// podNames := make([]string, 1)
	// for i := 0; i < 1; i++ {
	// 	pod.ObjectMeta.Name = name + ".pod." + strconv.Itoa(rand.Int())
	// 	podNames[i] = pod.ObjectMeta.Name
	// 	if err = cl.Create(context.TODO(), pod.DeepCopy()); err != nil {
	// 		t.Fatalf("create pod %d: (%v)", i, err)
	// 	}
	// }

	// //Reconcile again so Reconcile() checks pods and updates the MeterBase
	// //resources' Status.
	// res, err = r.Reconcile(req)
	// if err != nil {
	// 	t.Fatalf("reconcile: (%v)", err)
	// }
	// if res != (reconcile.Result{}) {
	// 	t.Error("reconcile did not return an empty Result")
	// }

	// // Get the updated MeterBase object.
	// meterbase = &marketplacev1alpha1.MeterBase{}
	// err = r.client.Get(context.TODO(), req.NamespacedName, meterbase)
	// if err != nil {
	// 	t.Errorf("get meterbase: (%v)", err)
	// }
}

func TestMeterBaseControllerFlags(t *testing.T) {
	flagset := FlagSet()

	if !flagset.HasFlags() {
		t.Errorf("no flags on flagset")
	}
}
