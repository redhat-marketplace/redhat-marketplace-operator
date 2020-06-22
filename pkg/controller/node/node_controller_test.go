package node

import (
	"testing"

	. "github.com/redhat-marketplace/redhat-marketplace-operator/test/rectest"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestNodeController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	t.Run("Test New Node", testNewNode)
	t.Run("Test Labels Absent", testNodeLabelsAbsent)
	t.Run("Test different Labels present", testNodeDiffLabelsPresent)
	t.Run("Test unknown Labels present", testNodeUnknown)
	t.Run("Test multiple nodes present", testMultipleNodes)
}

var (
	name             = "new-node"
	nameLabelsAbsent = "new-node-labels-absent"
	nameLabelsDiff   = "new-node-diff-labels"
	kind             = "Node"

	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: name,
		},
	}

	opts = []StepOption{
		WithRequest(req),
	}

	node = corev1.Node{
		TypeMeta: v1.TypeMeta{
			Kind: "Node",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				watchResourceTag: watchResourceValue,
			},
		},
		Spec: corev1.NodeSpec{},
	}

	nodeLabelsAbsent = corev1.Node{
		TypeMeta: v1.TypeMeta{
			Kind: "Node",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:   nameLabelsAbsent,
			Labels: map[string]string{},
		},
		Spec: corev1.NodeSpec{},
	}

	nodeLabelsDiff = corev1.Node{
		TypeMeta: v1.TypeMeta{
			Kind: "Node",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: nameLabelsDiff,
			Labels: map[string]string{
				"testKey": "testValue",
			},
		},
		Spec: corev1.NodeSpec{},
	}
)

func generateOpts(name string) []StepOption {
	return []StepOption{
		WithRequest(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: name,
			},
		}),
	}
}

func setup(r *ReconcilerTest) error {
	r.Client = fake.NewFakeClient(r.GetGetObjects()...)
	r.Reconciler = &ReconcileNode{client: r.Client, scheme: scheme.Scheme}
	return nil
}

func testNewNode(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, node.DeepCopyObject())
	reconcilerTest.TestAll(t,
		// Reconcile to create obj
		ReconcileStep(generateOpts(name),
			ReconcileWithExpectedResults(DoneResult)),
		// List and check results
		ListStep(opts,
			ListWithObj(&corev1.NodeList{}),
			ListWithFilter(
				client.MatchingLabels(map[string]string{
					watchResourceTag: watchResourceValue,
				})),
			ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
				list, ok := i.(*corev1.NodeList)

				assert.Truef(t, ok, "expected node list got type %T", i)
				assert.Equal(t, 1, len(list.Items))

			}),
		),
	)
}

func testNodeLabelsAbsent(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, nodeLabelsAbsent.DeepCopyObject())
	reconcilerTest.TestAll(t,
		// Reconcile to create obj
		ReconcileStep(generateOpts(nameLabelsAbsent),
			ReconcileWithExpectedResults(DoneResult)),
		// List and check results
		ListStep(opts,
			ListWithObj(&corev1.NodeList{}),
			ListWithFilter(
				client.MatchingLabels(map[string]string{
					watchResourceTag: watchResourceValue,
				})),
			ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
				list, ok := i.(*corev1.NodeList)

				assert.Truef(t, ok, "expected node list got type %T", i)
				assert.Equal(t, 1, len(list.Items))
				assert.Equal(t, watchResourceValue, list.Items[0].GetLabels()[watchResourceTag])
			}),
		),
	)
}

func testNodeDiffLabelsPresent(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, nodeLabelsDiff.DeepCopyObject())
	reconcilerTest.TestAll(t,
		// Reconcile to create obj
		ReconcileStep(generateOpts(nameLabelsDiff),
			ReconcileWithExpectedResults(DoneResult)),
		// List and check results
		ListStep(opts,
			ListWithObj(&corev1.NodeList{}),
			ListWithFilter(
				client.MatchingLabels(map[string]string{
					watchResourceTag: watchResourceValue,
				})),
			ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
				list, ok := i.(*corev1.NodeList)

				assert.Truef(t, ok, "expected node list got type %T", i)
				assert.Equal(t, 1, len(list.Items))
				assert.Equal(t, watchResourceValue, list.Items[0].GetLabels()[watchResourceTag])
				assert.Equal(t, "testValue", list.Items[0].GetLabels()["testKey"])
			}),
		),
	)
}

func testNodeUnknown(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, nodeLabelsDiff.DeepCopyObject())
	reconcilerTest.TestAll(t,
		// Reconcile to create obj
		ReconcileStep(generateOpts("DUMMY"),
			ReconcileWithExpectedResults(DoneResult)),
		// List and check results
		ListStep(opts,
			ListWithObj(&corev1.NodeList{}),
			ListWithFilter(
				client.MatchingLabels(map[string]string{
					watchResourceTag: watchResourceValue,
				})),
			ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
				list, ok := i.(*corev1.NodeList)

				assert.Truef(t, ok, "expected node list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			}),
		),
	)
}

func testMultipleNodes(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, nodeLabelsDiff.DeepCopyObject(), node.DeepCopyObject(), nodeLabelsAbsent.DeepCopyObject())
	reconcilerTest.TestAll(t,
		// Reconcile to create obj
		ReconcileStep(opts,
			ReconcileWithExpectedResults(DoneResult)),
		// List and check results
		ListStep([]StepOption{},
			ListWithObj(&corev1.NodeList{}),
			ListWithFilter(
				client.MatchingLabels(map[string]string{
					watchResourceTag: watchResourceValue,
				})),
			ListWithCheckResult(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
				list, ok := i.(*corev1.NodeList)

				assert.Truef(t, ok, "expected node list got type %T", i)
				assert.Equal(t, 1, len(list.Items))

				for _, item := range list.Items {
					assert.Equal(t, watchResourceValue, item.GetLabels()[watchResourceTag])
				}

			}),
		),
	)
}
