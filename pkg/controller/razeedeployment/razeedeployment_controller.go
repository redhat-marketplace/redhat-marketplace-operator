package razeedeployment

import (
	"context"
	batch "k8s.io/api/batch/v1"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/marketplace-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"k8s.io/apimachinery/pkg/api/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"k8s.io/apimachinery/pkg/types"
	// "github.com/operator-framework/operator-sdk/pkg/status"

)


var log = logf.Log.WithName("controller_razeedeployment")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new RazeeDeployment Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRazeeDeployment{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("razeedeployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource RazeeDeployment
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.RazeeDeployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// watch the Job type
	err = c.Watch(&source.Kind{Type: &batch.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.RazeeDeployment{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileRazeeDeployment implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileRazeeDeployment{}

// ReconcileRazeeDeployment reconciles a RazeeDeployment object
type ReconcileRazeeDeployment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a RazeeDeployment object and makes changes based on the state read
// and what is in the RazeeDeployment.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileRazeeDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RazeeDeployment")

	// Fetch the RazeeDeployment instance
	instance := &marketplacev1alpha1.RazeeDeployment{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if !instance.Spec.Enabled {
		reqLogger.Info("Razee not enabled")
		return reconcile.Result{}, nil
	}
	
	// Define a new razeedeploy-job object
	job := utils.MakeRazeeJob()

	// Check if the Job exists
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "razeedeploy-job",
			Namespace: "marketplace-operator",
		},
	}
	foundJob := &batch.Job{}
	err = r.client.Get(context.TODO(), req.NamespacedName, foundJob)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating razzeedeploy-job")
		err = r.client.Create(context.TODO(), job)
		if err != nil {
			reqLogger.Error(err,"Failed to create Job on cluster")
			return reconcile.Result{}, err
		}
		// Job created successfully - don't requeue
		reqLogger.Info("job created successfully")
		// reqque to grab the "foundJob"
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Job(s) from Cluster")
		return reconcile.Result{}, err
	}

	if err := controllerutil.SetControllerReference(instance, foundJob, r.scheme); err != nil {
		reqLogger.Error(err, "Failed to set controller reference")
		return reconcile.Result{}, err
	}

	// Update status and conditions
	instance.Status.JobState = foundJob.Status
	for _, jobCondition := range foundJob.Status.Conditions  {
		instance.Status.Conditions = jobCondition
	}
	
	err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update JobState.")
			return reconcile.Result{}, err
		}
	reqLogger.Info("Updated Status")

	// Namespace already exists - don't requeue
	// TODO: change this message here since we are requeueing
	// reqLogger.Info("Skip reconcile: Job already exists", "Namespace.Namespace", foundJob)
	return reconcile.Result{}, nil
}



