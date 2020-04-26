package razeedeployment

import (
	"context"

	// "reflect"
	"testing"
	"time"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	. "github.ibm.com/symposium/redhat-marketplace-operator/test/controller"
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
	t.Run("Test Old Install", testOldMigratedInstall)
}

func setup(r *ReconcilerTest) error {
	s := scheme.Scheme
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, razeeDeployment.DeepCopy(), &marketplacev1alpha1.RazeeDeploymentList{})
	r.SetClient(fake.NewFakeClient(r.GetRuntimeObjects()...))
	r.SetReconciler(&ReconcileRazeeDeployment{client: r.GetClient(), scheme: s, opts: &RazeeOpts{RazeeJobImage: "test"}})
	return nil
}

var (
	name      = "marketplaceconfig"
	namespace = "openshift-redhat-marketplace"
	secretName = "rhm-operator-secret"

	opts      = []TestCaseOption{
		WithRequest(req),
		WithNamespace("openshift-redhat-marketplace"),
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
			Enabled:          true,
			ClusterUUID:      "foo",
			DeploySecretName: &secretName,
		},
	}
	namespObj = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	razeeNsObj = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "razee",
		},
	}
	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhm-operator-secret",
			Namespace: "redhat-marketplace-operator",
		},
		Data: map[string][]byte{
			IBM_COS_READER_KEY_FIELD: []byte("ibm-cos-reader-key"),
			IBM_COS_URL_FIELD:        []byte("ibm-cos-url"),
			BUCKET_NAME_FIELD:        []byte("bucket-name"),
			RAZEE_DASH_ORG_KEY_FIELD: []byte("razee-dash-org-key"),
			CHILD_RRS3_YAML_FIELD:    []byte("childRRS3-filename"),
			RAZEE_DASH_URL_FIELD:     []byte("razee-dash-url"),
			FILE_SOURCE_URL_FIELD:    []byte("file-source-url"),
		},
	}
)

func testCleanInstall(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup,
		&razeeDeployment,
		&secret,
		&namespObj,
	)
	reconcilerTest.TestAll(t,
		[]TestCaseStep{
			NewReconcilerTestCase(
				append(
					opts,
					WithName("rhm-operator-secret"),
					WithNamespace(namespace),
					WithExpectedResult(reconcile.Result{RequeueAfter: time.Second * 30}),
				)...,
			),
			NewReconcilerTestCase(
				append(opts,
					WithName(namespace),
					WithNamespace(""),
					WithExpectedResult(reconcile.Result{}),
					WithTestObj(&corev1.Namespace{}))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.ConfigMap{}),
					WithName("watch-keeper-non-namespaced"),
					WithAfter(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
						watchKeeperNonNamespace, ok := i.(*corev1.ConfigMap)
						if !ok {
							t.Fatalf("Type is not expected %T", i)
						}

						razeeController := ReconcileRazeeDeployment{}
						expectedWatchKeeperNonNamespace := razeeController.MakeWatchKeeperNonNamespace()

						patchResult, err := patch.DefaultPatchMaker.Calculate(watchKeeperNonNamespace, expectedWatchKeeperNonNamespace)
						if !patchResult.IsEmpty() {
							t.Fatalf("Discrepency on object %T", patchResult)
						}

						if err != nil {
							t.Fatalf("Error calculating patch %T", patchResult)
						}
					}),
					)...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.ConfigMap{}),
					WithName("watch-keeper-limit-poll"),
					WithAfter(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
						watchKeeperLimitPoll, ok := i.(*corev1.ConfigMap)
						if !ok {
							t.Fatalf("Type is not expected %T", i)
						}

						razeeController := ReconcileRazeeDeployment{}
						expectedWatchKeeperLimitPoll := razeeController.MakeWatchKeeperLimitPoll()

						patchResult, err := patch.DefaultPatchMaker.Calculate(watchKeeperLimitPoll, expectedWatchKeeperLimitPoll)
						if !patchResult.IsEmpty() {
							t.Fatalf("Discrepency on object %T", patchResult)
						}

						if err != nil {
							t.Fatalf("Error calculating patch %T", patchResult)
						}
					}),
					)...),
			NewReconcilerTestCase(
				append(
					opts,
					WithTestObj(&corev1.ConfigMap{}),
					WithName("razee-cluster-metadata"),
					WithAfter(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
						razeeClusterMetadata, ok := i.(*corev1.ConfigMap)

						if !ok {
							t.Fatalf("Type is not expected %T", i)
						}

						razeeController := ReconcileRazeeDeployment{}
						expectedRazeeClusterMetadata := razeeController.MakeRazeeClusterMetaData(&razeeDeployment)

						patchResult, err := patch.DefaultPatchMaker.Calculate(razeeClusterMetadata, expectedRazeeClusterMetadata)
						if !patchResult.IsEmpty() {
							t.Fatalf("Discrepency on object %T", patchResult)
						}

						if err != nil {
							t.Fatalf("Error calculating patch %T", patchResult)
						}

					}),
					)...
				),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.ConfigMap{}),
					WithName("watch-keeper-config"),
					
					)...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.Secret{}),
					WithName("watch-keeper-secret"),
					)...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&corev1.Secret{}),
					WithName("ibm-cos-reader-key"),
					WithAfter(func(r *ReconcilerTest, t *testing.T, i runtime.Object) {
						ibmCosReaderKey, ok := i.(*corev1.Secret)
						if !ok {
							t.Fatalf("Type is not expected %T", i)
						}

						razeeDeployment.Spec.DeployConfig = &marketplacev1alpha1.RazeeConfigurationValues{}
						razeeDeployment.Spec.DeployConfig.IbmCosReaderKey = &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "rhm-operator-secret",
							},
							Key: IBM_COS_READER_KEY_FIELD,
						}

						razeeController := ReconcileRazeeDeployment{}
						expectedIbmCosReaderKey,err := razeeController.MakeCOSReaderSecret(&razeeDeployment,req)

						patchResult, err := patch.DefaultPatchMaker.Calculate(ibmCosReaderKey, &expectedIbmCosReaderKey)
						if !patchResult.IsEmpty() {
							t.Fatalf("Discrepency on object %T", patchResult)
						}

						if err != nil {
							t.Fatalf("Error calculating patch %T", patchResult)
						}
					}),
					)...),
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

var (
	oldRazeeDeployment = marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled:     true,
			ClusterUUID: "foo",
		},
		Status: marketplacev1alpha1.RazeeDeploymentStatus{
			RazeeJobInstall: &marketplacev1alpha1.RazeeJobInstallStruct{
				RazeeNamespace: "razee",
			},
		},
	}
)

func testOldMigratedInstall(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup,
		&oldRazeeDeployment,
		&secret,
		&namespObj,
		&razeeNsObj,
	)
	reconcilerTest.TestAll(t,
		[]TestCaseStep{
			NewReconcilerTestCase(
				append(opts,
					WithName(namespace),
					WithNamespace(""),
					WithExpectedResult(reconcile.Result{}),
					WithTestObj(&corev1.Namespace{}))...),
			NewReconcilerTestCase(
				append(opts,
					WithNamespace("razee"),
					WithTestObj(&corev1.ConfigMap{}),
					WithName("watch-keeper-non-namespaced"))...),
			NewReconcilerTestCase(
				append(opts,
					WithNamespace("razee"),
					WithTestObj(&corev1.ConfigMap{}),
					WithName("watch-keeper-limit-poll"))...),
			NewReconcilerTestCase(
				append(opts,
					WithNamespace("razee"),
					WithTestObj(&corev1.ConfigMap{}),
					WithName("razee-cluster-metadata"))...),
			NewReconcilerTestCase(
				append(opts,
					WithNamespace("razee"),
					WithTestObj(&corev1.ConfigMap{}),
					WithName("watch-keeper-config"))...),
			NewReconcilerTestCase(
				append(opts,
					WithNamespace("razee"),
					WithTestObj(&corev1.Secret{}),
					WithName("watch-keeper-secret"))...),
			NewReconcilerTestCase(
				append(opts,
					WithNamespace("razee"),
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
					WithExpectedResult(reconcile.Result{RequeueAfter: time.Second * 30}),
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

// func TransferMetadata( in runtime.Object, out runtime.Object)(runtime.Object){
	
// 	out.SetAnnotations(in.GetAnnotations())
// 	out.SetCreationTimestamp(in.GetCreationTimestamp())
// 	out.SetFinalizers(in.GetFinalizers())
// 	out.SetGeneration(in.GetGeneration())
// 	out.SetResourceVersion(in.GetResourceVersion())
// 	out.SetSelfLink(in.GetSelfLink())
// 	out.SetUID(in.GetUID())

// 	return out
// }
