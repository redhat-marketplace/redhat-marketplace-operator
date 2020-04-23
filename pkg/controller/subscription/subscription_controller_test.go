package subscription

import (
	"testing"

	. "github.ibm.com/symposium/redhat-marketplace-operator/test/controller"

	"context"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	opsrcApi "github.com/operator-framework/operator-marketplace/pkg/apis"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestSubscriptionController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	defaultFeatures := []string{"razee", "meterbase"}
	viper.Set("assets", "../../../assets")
	viper.Set("features", defaultFeatures)

	t.Run("Test Nub Subscription", testNewSubscription)
	t.Run("Test New Sub with Existing OG", testNewSubscriptionWithOperatorGroup)
	t.Run("Test Sub with OG Added", testDeleteOperatorGroupIfTooMany)
}

var (
	name      = "new-subscription"
	namespace = "arbitrary-namespace"

	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	opts = []TestCaseOption{
		WithRequest(req),
		WithNamespace(namespace),
		WithName(name),
	}
	preExistingOperatorGroup = &olmv1.OperatorGroup{
		ObjectMeta: v1.ObjectMeta{
			Name:      "existing-group",
			Namespace: namespace,
		},
		Spec: olmv1.OperatorGroupSpec{
			TargetNamespaces: []string{
				"arbitrary-namespace",
			},
		},
	}
	subscription = &olmv1alpha1.Subscription{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				operatorTag: "true",
			},
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			CatalogSource:          "source",
			CatalogSourceNamespace: "source-namespace",
			Package:                "source-package",
		},
	}
)

func setup(r *ReconcilerTest) error {
	s := scheme.Scheme
	_ = opsrcApi.AddToScheme(s)
	_ = olmv1alpha1.AddToScheme(s)
	_ = olmv1.AddToScheme(s)

	r.Client = fake.NewFakeClient(r.GetRuntimeObjects()...)
	r.Reconciler = &ReconcileSubscription{client: r.Client, scheme: s}
	return nil
}

func testNewSubscription(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, subscription)
	reconcilerTest.TestAll(t,
		[]TestCaseStep{
			NewReconcilerTestCase(
				append(opts,
					WithExpectedResult(reconcile.Result{Requeue: true}),
					WithTestObj(&olmv1.OperatorGroupList{}),
					WithLabels(map[string]string{
						operatorTag: "true",
					}))...),
			NewReconcileStep(append(opts, WithExpectedResult(reconcile.Result{}))...),
			NewClientListStep(append(opts,
				WithTestObj(&olmv1.OperatorGroupList{}),
				WithLabels(map[string]string{
					operatorTag: "true",
				}),
				WithAfter(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
					list, ok := i.(*olmv1.OperatorGroupList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 1, len(list.Items))
				}),
			)...),
		})
}

func testNewSubscriptionWithOperatorGroup(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, subscription, preExistingOperatorGroup)
	reconcilerTest.TestAll(t,
		[]TestCaseStep{
			NewReconcileStep(append(opts, WithExpectedResult(reconcile.Result{}))...),
			NewClientListStep(append(opts, WithTestObj(&olmv1.OperatorGroupList{}),
				WithAfter(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
					list, ok := i.(*olmv1.OperatorGroupList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 1, len(list.Items))
				}),
			)...),
		})
}

func testDeleteOperatorGroupIfTooMany(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, subscription)
	reconcilerTest.TestAll(t,
		[]TestCaseStep{
			NewReconcileStep(append(opts, WithExpectedResult(reconcile.Result{Requeue: true}))...),
			NewReconcileStep(append(opts, WithExpectedResult(reconcile.Result{Requeue: false}))...),
			NewClientListStep(append(opts, WithTestObj(&olmv1.OperatorGroupList{}),
				WithAfter(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
					list, ok := i.(*olmv1.OperatorGroupList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 1, len(list.Items))

					r.GetClient().Create(context.TODO(), preExistingOperatorGroup)
				}),
			)...),
			NewClientListStep(append(opts, WithTestObj(&olmv1.OperatorGroupList{}),
				WithAfter(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
					list, ok := i.(*olmv1.OperatorGroupList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 2, len(list.Items))
				}),
			)...),
			NewReconcileStep(append(opts, WithExpectedResult(reconcile.Result{Requeue: true}))...),
			NewReconcileStep(append(opts, WithExpectedResult(reconcile.Result{Requeue: false}))...),
			NewClientListStep(append(opts, WithTestObj(&olmv1.OperatorGroupList{}),
				WithAfter(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
					list, ok := i.(*olmv1.OperatorGroupList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 1, len(list.Items))
				}),
			)...),
		})
}
