package razeedeployment

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/marketplace-operator/pkg/utils"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	DEFAULT_RAZEE_JOB_IMAGE = "quay.io/razee/razeedeploy-delta:0.3.1"
	DEFAULT_RAZEEDASH_URL   = "http://169.45.231.109:8081/api/v2"
)

var (
	log          = logf.Log.WithName("controller_razeedeployment")
	razeeFlagSet *pflag.FlagSet
)

func init() {
	razeeFlagSet = pflag.NewFlagSet("razee", pflag.ExitOnError)
	razeeFlagSet.String("razee-job-image", DEFAULT_RAZEE_JOB_IMAGE, "image for the razee job")
	razeeFlagSet.String("razeedash-url", DEFAULT_RAZEEDASH_URL, "url that watch keeper posts data too")
}

func FlagSet() *pflag.FlagSet {
	return razeeFlagSet
}

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

type RazeeOpts struct {
	RazeeDashUrl  string
	RazeeJobImage string
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

	// if not enabled then exit
	if !instance.Spec.Enabled {
		reqLogger.Info("Razee not enabled")
		return reconcile.Result{}, nil
	}


	// if the razeejob hasn't been successfully created yet continue with reconcile
	if instance.Status.JobState.Succeeded == 1  {
		reqLogger.Info("Exiting reconcile loop - RazeeDeployJob has already been successfully created")
		return reconcile.Result{}, nil
	}

	secrets := &corev1.SecretList{}
	listOpts := []client.ListOption{
		client.InNamespace("razee"),
	}
	err = r.client.List(context.TODO(), secrets, listOpts...)
	reqLogger.Info("looking for secrets in razee")
	if err != nil{
		reqLogger.Error(err, "Failed to list secrets")
	}
	
	secretNames := utils.GetSecretNames(secrets.Items)
	for _, name := range secretNames{
		out := fmt.Sprintf("%v\n", name)
		fmt.Println(out)
	}

	// look for the prerequites and update status accordingly
	// TODO: double check that this is the extensive list
	searchItems := []string{"watch-keeper-secret","ibm-cos-reader-key"}
	if missing := utils.ContainsMultiple(secretNames,searchItems);len(missing)>0 {
		reqLogger.Info("There are missing prerequisite resources")
		
		//TODO: update the status with any of the missing resources (prerequisites)
		for _, item := range missing{
			reqLogger.Info("mssing resource","item: ", item)
			instance.Status.MissingRazeeResources = append(instance.Status.MissingRazeeResources, item)
		}
		reqLogger.Info("updating status with missing resources")
		fmt.Println(instance.Status)
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil{
			reqLogger.Error(err, "Error updating status with missing razee resources")
			// TODO: requeue here ? 
			// return reconcile.Result{}, err
		}
		reqLogger.Info("status has been updated with missing resources")
	}

	// Define a new razeedeploy-job object
	razeeOpts := &RazeeOpts{
		RazeeDashUrl:  viper.GetString("razeedash-url"),
		RazeeJobImage: viper.GetString("razee-job-image"),
	}

	job := r.MakeRazeeJob(razeeOpts)

	// Check if the Job exists already
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "razeedeploy-job",
			Namespace: "marketplace-operator",
		},
	}

	foundJob := &batch.Job{}
	err = r.client.Get(context.TODO(), req.NamespacedName, foundJob)

	// if the job doesn't exist create it
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating razzeedeploy-job")
		err = r.client.Create(context.TODO(), job)
		if err != nil {
			reqLogger.Error(err, "Failed to create Job on cluster")
			return reconcile.Result{}, err
		}
		reqLogger.Info("job created successfully")
		// requeue to grab the "foundJob" and continue to update status
		// wait 30 seconds so the job has time to complete
		// not entirely necessary, but the struct on Status.Conditions needs the Conditions in the job to be populated.
		return reconcile.Result{RequeueAfter: time.Second * 30}, nil
		// return reconcile.Result{Requeue: true}, nil
		// return reconcile.Result{}, nil
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
	for _, jobCondition := range foundJob.Status.Conditions {
		instance.Status.Conditions = jobCondition
	}

	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update JobState.")
		return reconcile.Result{}, err
	}
	reqLogger.Info("Updated Status")

	// if the job has a status of succeeded, then delete the job
	if foundJob.Status.Succeeded == 1 {
		err = r.client.Delete(context.TODO(), foundJob)
		if err != nil {
			reqLogger.Error(err, "Failed to delete job")
			return reconcile.Result{RequeueAfter: time.Second * 30}, nil
		}
		reqLogger.Info("Razeedeploy-job deleted")
		// exit the loop after the job has been deleted
		return reconcile.Result{}, nil
	}

	reqLogger.Info("End of reconcile")
	return reconcile.Result{}, nil
}

func (r *ReconcileRazeeDeployment) MakeRazeeJob(opt *RazeeOpts) *batch.Job {

	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "razeedeploy-job",
			Namespace: "marketplace-operator",
		},
		Spec: batch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "marketplace-operator",
					Containers: []corev1.Container{{
						Name:    "razeedeploy-job",
						Image:   opt.RazeeJobImage,
						Command: []string{"node", "src/install", "--namespace=razee"},
						Args:    []string{fmt.Sprintf("--razeedash-url=%v", opt.RazeeDashUrl)},
					}},
					RestartPolicy: "Never",
				},
			},
		},
	}
}
