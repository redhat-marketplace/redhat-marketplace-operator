package meterreport

import (
	"context"
	"reflect"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
)

var log = logf.Log.WithName("controller_meterreport")

// Add creates a new MeterReport Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
	cfg *config.OperatorConfig,
) error {
	return add(mgr, newReconciler(mgr, ccprovider, cfg))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
	cfg *config.OperatorConfig,
) reconcile.Reconciler {
	return &ReconcileMeterReport{
		client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		ccprovider: ccprovider,
		cfg:        cfg,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("meterreport-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MeterReport
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MeterReport{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner MeterReport
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterReport{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMeterReport implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMeterReport{}

// ReconcileMeterReport reconciles a MeterReport object
type ReconcileMeterReport struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	cfg        *config.OperatorConfig
	ccprovider ClientCommandRunnerProvider
}

// Reconcile reads that state of the cluster for a MeterReport object and makes changes based on the state read
// and what is in the MeterReport.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMeterReport) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterReport")

	cc := r.ccprovider.NewCommandRunner(r.client, r.scheme, reqLogger)

	// Fetch the MeterReport instance
	instance := &marketplacev1alpha1.MeterReport{}

	if result, _ := cc.Do(context.TODO(), GetAction(request.NamespacedName, instance)); !result.Is(Continue) {
		if result.Is(NotFound) {
			reqLogger.Info("MeterReport resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}

		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get MeterReport.")
		}

		return result.Return()
	}

	c := manifests.NewOperatorConfig(r.cfg)
	factory := manifests.NewFactory(instance.Namespace, c)

	job, err := factory.ReporterJob(instance)

	reqLogger.Info("config",
		"config", r.cfg.Image,
		"envvar", utils.Getenv("RELATED_IMAGE_REPORTER", ""),
		"job", job)

	if err != nil {
		reqLogger.Error(err, "failed to get job template")
		return reconcile.Result{}, err
	}

	if result, _ := cc.Do(
		context.TODO(),
		HandleResult(
			GetAction(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, job),
			OnNotFound(CreateAction(job, CreateWithAddOwner(instance))),
		),
	); !result.Is(Continue) {
		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get create job.")
		}

		return result.Return()
	}

	jr := &common.JobReference{}
	jr.SetFromJob(job)

	if result, _ := cc.Do(context.TODO(), Call(r.updateStatus(instance, jr))); !result.Is(Continue) {
		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get update status.")
		}

		return result.Return()
	}

	reqLogger.Info("reconcile finished")
	return reconcile.Result{}, nil
}

func (r *ReconcileMeterReport) updateStatus(
	instance *marketplacev1alpha1.MeterReport,
	jobRef *common.JobReference,
) func() (ClientAction, error) {
	return func() (ClientAction, error) {
		reqLogger := log.WithValues("Instance.Namespace", instance.Namespace, "Instance.Name", instance.Name)

		if !reflect.DeepEqual(instance.Status.AssociatedJob, jobRef) {
			instance.Status.AssociatedJob = jobRef

			reqLogger.Info("Updating MeterReport status associatedJob")

			return UpdateAction(instance, UpdateStatusOnly(true)), nil
		}

		return nil, nil
	}
}
