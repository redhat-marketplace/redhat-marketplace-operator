package razeedeployment

import (
	"context"
	"testing"
	"time"

	. "github.ibm.com/symposium/redhat-marketplace-operator/test/controller"

	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
func TestRazeeDeployController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	viper.Set("assets", "../../../assets")
 
	t.Run("Test Clean Install", testCleanInstall)
	t.Run("Test No Secret", testNoSecret)
}

func setup(r *ReconcilerTest) error {
	s := scheme.Scheme
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, razeeDeployment.DeepCopy())
	r.SetClient(fake.NewFakeClient(r.GetRuntimeObjects()...))
	r.SetReconciler(&ReconcileRazeeDeployment{client: r.GetClient(), scheme: s, opts: &RazeeOpts{RazeeJobImage: "test"}})
	return nil
}

var (
	name      = "marketplaceconfig"
	namespace = "redhat-marketplace-operator"
	opts      = []TestCaseOption{
		WithRequest(req),
		WithNamespace(RAZEE_NAMESPACE),
		WithName(name),
	}
	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	razeeDeployment = marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled:     true,
			ClusterUUID: "foo",
		},
	}
	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhm-operator-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			IBM_COS_READER_KEY_FIELD: []byte("test"),
			IBM_COS_URL_FIELD:        []byte("test"),
			BUCKET_NAME_FIELD:        []byte("test"),
			RAZEE_DASH_ORG_KEY_FIELD: []byte("test"),
			CHILD_RRS3_YAML_FIELD:    []byte("test"),
			RAZEE_DASH_URL_FIELD:     []byte("test"),
			FILE_SOURCE_URL_FIELD:    []byte("test"),
		},
	}
)

func testCleanInstall(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup,
		&razeeDeployment,
		&secret)
	reconcilerTest.TestAll(t,
		[]TestCaseStep{
			NewReconcilerTestCase(
				append(opts,
					WithName("razee"),
					WithNamespace(""),
					WithTestObj(&corev1.Namespace{}))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.ConfigMap{}),
					WithName("watch-keeper-non-namespaced"))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.ConfigMap{}),
					WithName("watch-keeper-limit-poll"))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.ConfigMap{}),
					WithName("razee-cluster-metadata"))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.ConfigMap{}),
					WithName("watch-keeper-config"))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.Secret{}),
					WithName("watch-keeper-secret"))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.Secret{}),
					WithName("rhm-cos-reader-key"))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&batch.Job{}),
					WithName("razeedeploy-job"),
					WithNamespace(namespace),
					WithExpectedResult(reconcile.Result{RequeueAfter: time.Second * 30}),
					WithAfter(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
						myJob, ok := i.(*batch.Job)

						if !ok {
							t.Fatalf("Type is not expected %T", i)
						}

						myJob.Status.Conditions = []batch.JobCondition{
							batch.JobCondition{
								Type:               batch.JobComplete,
								Status:             corev1.ConditionTrue,
								LastProbeTime:      metav1.Now(),
								LastTransitionTime: metav1.Now(),
								Reason:             "Job is complete",
								Message:            "Job is complete",
							},
						}

						r.Client.Status().Update(context.TODO(), myJob)
					}))...),
			NewReconcilerTestCase(
				append(opts,
					WithName("razeedeploy-job"),
					WithExpectedResult(reconcile.Result{}),
					WithTestObj(nil))...),
		})
}

func testNoSecret(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, &razeeDeployment)
	reconcilerTest.TestAll(t,
		[]TestCaseStep{
			NewReconcilerTestCase(
				append(opts,
					WithName("rhm-operator-secret"),
					WithNamespace(namespace),
					WithExpectedResult(reconcile.Result{}),
					WithExpectedError(nil))...),
		})
}

func CreateWatchKeeperSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "razeedeploy-job",
			Namespace: "marketplace-operator",
		},
	}
}
