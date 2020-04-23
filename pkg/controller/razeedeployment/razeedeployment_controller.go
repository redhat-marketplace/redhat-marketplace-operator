package razeedeployment

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/utils"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	razeeDeploymentFinalizer   = "razeedeploy.finalizer.marketplace.redhat.com"
	RAZEE_UNINSTALL_NAME       = "razee-uninstall-job"
	DEFAULT_RAZEE_JOB_IMAGE    = "quay.io/razee/razeedeploy-delta:1.1.0"
	DEFAULT_RAZEEDASH_URL      = `http://169.45.231.109:8081/api/v2`
	WATCH_KEEPER_VERSION       = "0.5.0"
	FEATURE_FLAG_VERSION       = "0.6.1"
	MANAGED_SET_VERSION        = "0.4.2"
	MUSTACHE_TEMPLATE_VERSION  = "0.6.3"
	REMOTE_RESOURCE_VERSION    = "0.4.2"
	REMOTE_RESOURCE_S3_VERSION = "0.5.2"
	IBM_COS_READER_KEY_FIELD   = "IBM_COS_READER_KEY"
	BUCKET_NAME_FIELD          = "BUCKET_NAME"
	IBM_COS_URL_FIELD          = "IBM_COS_URL"
	RAZEE_DASH_ORG_KEY_FIELD   = "RAZEE_DASH_ORG_KEY"
	CHILD_RRS3_YAML_FIELD      = "CHILD_RRS3_YAML_FILENAME"
	RAZEE_DASH_URL_FIELD       = "RAZEE_DASH_URL"
	FILE_SOURCE_URL_FIELD      = "FILE_SOURCE_URL"
	RHM_OPERATOR_SECRET_NAME   = "rhm-operator-secret"
	RAZEE_NAMESPACE            = "razee"
	RAZEE_DEPLOY_JOB           = "razeedeploy-job"
)

var (
	log                     = logf.Log.WithName("controller_razeedeployment")
	razeeFlagSet            *pflag.FlagSet
	RELATED_IMAGE_RAZEE_JOB = "RELATED_IMAGE_RAZEE_JOB"
	clusterUUID             = ""
)

func init() {
	razeeFlagSet = pflag.NewFlagSet("razee", pflag.ExitOnError)
	razeeFlagSet.String("razee-job-image", utils.Getenv(RELATED_IMAGE_RAZEE_JOB, DEFAULT_RAZEE_JOB_IMAGE), "image for the razee job")
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
	razeeOpts := &RazeeOpts{
		RazeeJobImage: viper.GetString("razee-job-image"),
	}

	return &ReconcileRazeeDeployment{client: mgr.GetClient(), scheme: mgr.GetScheme(), opts: razeeOpts}
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

	secretPred := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Meta.GetName() == RHM_OPERATOR_SECRET_NAME
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Meta.GetName() == RHM_OPERATOR_SECRET_NAME
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.MetaNew.GetName() == RHM_OPERATOR_SECRET_NAME
		},
	}

	err = c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestForObject{},
		secretPred,
	)
	if err != nil {
		return err
	}

	jobPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Meta.GetName() == RAZEE_DEPLOY_JOB
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Meta.GetName() == RAZEE_DEPLOY_JOB
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.MetaOld.GetName() == RAZEE_DEPLOY_JOB
		},
	}

	err = c.Watch(
		&source.Kind{Type: &batch.Job{}},
		&handler.EnqueueRequestForObject{},
		jobPredicate,
	)
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
	opts   *RazeeOpts
}

type RazeeOpts struct {
	RazeeJobImage string
	ClusterUUID   string
}

// Reconcile reads that state of the cluster for a RazeeDeployment object and makes changes based on the state read
// and what is in the RazeeDeployment.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileRazeeDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// MissingDeploySecretValues := make([]string, 0, 8)
	// localSecretVarsPopulated := false

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RazeeDeployment")

	switch request.Name {
	case RHM_OPERATOR_SECRET_NAME:
		_, err := r.reconcileRhmOperatorSecret(&request)
		if err != nil {
			reqLogger.Error(err, "Failed to reconcile secret")
		}

	case RAZEE_DEPLOY_JOB:
		reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
		reqLogger.Info("Beginning of Razeedeploy-job controller Instance reconciler")
		// Fetch the RazeeDeployment instance
		instance := &marketplacev1alpha1.RazeeDeployment{}
		err := r.client.Get(context.TODO(), types.NamespacedName{
			Name:      "rhm-marketplaceconfig-razeedeployment",
			Namespace: "redhat-marketplace-operator",
		}, instance)
		if err != nil {
			if errors.IsNotFound(err) {
				// Request object not found, could have been deleted after reconcile request.
				// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
				// Return and don't requeue
				reqLogger.Error(err, "Failed to find RazeeDeployment instance")
				return reconcile.Result{}, nil
			}
			// Error reading the object - requeue the request.
			return reconcile.Result{}, err
		}

		foundJob := batch.Job{}
		err = r.client.Get(context.TODO(), request.NamespacedName, &foundJob)
		if err != nil {
			reqLogger.Error(err, "failed to find razee deploy job")
		}

		// if the conditions have populated then update status
		if len(foundJob.Status.Conditions) != 0 {
			reqLogger.Info("RazeeJob Conditions have been propagated")
			// Update status and conditions
			instance.Status.JobState = foundJob.Status
			for _, jobCondition := range foundJob.Status.Conditions {
				instance.Status.Conditions = &jobCondition
			}
			instance.Status.RazeeJobInstall = &marketplacev1alpha1.RazeeJobInstallStruct{
				RazeeNamespace:  RAZEE_NAMESPACE,
				RazeeInstallURL: instance.Spec.DeployConfig.FileSourceURL,
			}
		}

		// delete the job after it's successful
		if foundJob.Status.Succeeded == 1 {
			reqLogger.Info("Deleting Razee Job")
			err = r.client.Delete(context.TODO(), &foundJob)
			if err != nil {
				reqLogger.Error(err, "Failed to delete job")
				return reconcile.Result{}, err
			}

			reqLogger.Info("Razeedeploy-job deleted")
		}

		reqLogger.Info("updating status inside job reconciler")
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update JobState")
			return reconcile.Result{}, nil
		}
		reqLogger.Info("Updated JobState")
		reqLogger.Info("End of razee job reconciler")

	default:
		reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
		reqLogger.Info("Beginning of RazeeDeploy Instance reconciler")
		// Fetch the RazeeDeployment instance
		instance := &marketplacev1alpha1.RazeeDeployment{}
		err := r.client.Get(context.TODO(), request.NamespacedName, instance)
		if err != nil {
			if errors.IsNotFound(err) {
				// Request object not found, could have been deleted after reconcile request.
				// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
				// Return and don't requeue
				reqLogger.Error(err, "Failed to find RazeeDeployment instance")
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

		// Adding a finalizer to this CR
		if !utils.Contains(instance.GetFinalizers(), razeeDeploymentFinalizer) {
			if err := r.addFinalizer(instance, request.Namespace); err != nil {
				return reconcile.Result{}, err
			}
		}

		// Check if the RazeeDeployment instance is being marked for deletion
		isMarkedForDeletion := instance.GetDeletionTimestamp() != nil
		if isMarkedForDeletion {
			if utils.Contains(instance.GetFinalizers(), razeeDeploymentFinalizer) {
				//Run finalization logic for the razeeDeploymentFinalizer.
				//If it fails, don't remove the finalizer so we can retry during the next reconcile
				return r.finalizeRazeeDeployment(instance)
			}
			return reconcile.Result{}, nil
		}

		/******************************************************************************
		PROCEED WITH CREATING RAZEE PREREQUISITES? YES/NO
		do we have all the fields from rhm-secret ? (combined secret)
		check that we can continue with applying the razee job
		if the job has already run exit
		if there are still missing resources exit
		/******************************************************************************/
		if instance.Spec.DeployConfig == nil {
			reqLogger.Info("rhm-operator-secret has not been applied")
			req := reconcile.Request{
				types.NamespacedName{
					Name:      *instance.Spec.DeploySecretName,
					Namespace: request.Namespace,
				},
			}
			r.reconcileRhmOperatorSecret(&req)
			return reconcile.Result{RequeueAfter: time.Second * 30}, nil
		}

		if instance.Spec.DeployConfig != nil {
			if len(instance.Status.MissingDeploySecretValues) > 0 {
				reqLogger.Info("Missing required razee configuration values")

				req := reconcile.Request{
					types.NamespacedName{
						Name:      *instance.Spec.DeploySecretName,
						Namespace: request.Namespace,
					},
				}
				_, err := r.reconcileRhmOperatorSecret(&req)
				if err != nil {
					reqLogger.Error(err, "Failed to reconcile secret")
				}
				return reconcile.Result{RequeueAfter: time.Second * 30}, nil
			} else {
				reqLogger.Info("all secret values found")
				//TODO: I could maybe move this down into MakeParentRemoteResource()
				//construct the childURL
				url := fmt.Sprintf("%s/%s/%s/%s", instance.Spec.DeployConfig.IbmCosURL, instance.Spec.DeployConfig.BucketName, instance.Spec.ClusterUUID, instance.Spec.DeployConfig.ChildRSSFIleName)
				instance.Spec.ChildUrl = &url
				fmt.Println("CHILD URL", url)
				err = r.client.Update(context.TODO(), instance)
				if err != nil {
					reqLogger.Error(err, "Failed to update ChildUrl")
				}
				reqLogger.Info("All required razee configuration values have been found")
			}

		}

		/******************************************************************************
		APPLY OR OVERWRITE RAZEE RESOURCES
		/******************************************************************************/
		newResources := []string{}
		razeeNamespace := corev1.Namespace{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "razee"}, &razeeNamespace)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info("razee namespace does not exist - creating")
				razeeNamespace.ObjectMeta.Name = "razee"
				err = r.client.Create(context.TODO(), &razeeNamespace)
				if err != nil {
					reqLogger.Error(err, "Failed to create razee namespace.")
				}
				return reconcile.Result{Requeue: true}, nil
			} else {
				reqLogger.Error(err, "Failed to get razee ns.")
				return reconcile.Result{}, err
			}
		}
		if &razeeNamespace != nil {
			reqLogger.Info("razee namespace already exists")
		}

		newResources = append(newResources, fmt.Sprintf("%v namespace", razeeNamespace.Name))
	
		// update status
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		instance.Status.RazeePrerequisitesCreated = newResources
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}
	
		watchKeeperNonNamespace := corev1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-non-namespaced", Namespace: "razee"}, &watchKeeperNonNamespace)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info("watch-keeper-non-namespace does not exist - creating")

				// set the annotation
				watchKeeperNonNamespace = *r.MakeWatchKeeperNonNamespace()
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&watchKeeperNonNamespace); err != nil {
					reqLogger.Error(err, "Failed to set annotation")
				}

				err = r.client.Create(context.TODO(), &watchKeeperNonNamespace)
				if err != nil {
					reqLogger.Error(err, "Failed to create ", "resource: ", watchKeeperNonNamespace.Name)
					return reconcile.Result{}, err
				}
				return reconcile.Result{Requeue: true}, nil
			} else {
				reqLogger.Error(err, "Failed to get ", "resource: ", watchKeeperNonNamespace.Name)
				return reconcile.Result{}, err
			}
		}
		if err == nil {
			reqLogger.Info(fmt.Sprintf("Resource already exists %v", watchKeeperNonNamespace.Name))
			// proposed change
			updatedWatchKeeperNonNameSpace := *r.MakeWatchKeeperNonNamespace()
			patchResult, err := patch.DefaultPatchMaker.Calculate(&watchKeeperNonNamespace, &updatedWatchKeeperNonNameSpace)
			if err != nil {
				// handle the error
				reqLogger.Error(err, "Failed to compare patches")
			}

			if !patchResult.IsEmpty() {
				reqLogger.Info(fmt.Sprintf("Change detected on %v", watchKeeperNonNamespace.Name))
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&updatedWatchKeeperNonNameSpace); err != nil {

				}
				reqLogger.Info("Updating resource", "resource: ", watchKeeperNonNamespace.Name)
				err = r.client.Update(context.TODO(), &updatedWatchKeeperNonNameSpace)
				if err != nil {
					reqLogger.Info(fmt.Sprintf("Failed to update %v", watchKeeperNonNamespace.Name))
					return reconcile.Result{}, err
				}
			}

			reqLogger.Info(fmt.Sprintf("No change detected on %v", watchKeeperNonNamespace.Name))

		}

		newResources = append(newResources,watchKeeperNonNamespace.Name)
	
		// update status
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		instance.Status.RazeePrerequisitesCreated = newResources
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}
		
		// apply watch-keeper-limit-poll config map
		watchKeeperLimitPoll := corev1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-limit-poll", Namespace: "razee"}, &watchKeeperLimitPoll)
		if err != nil {
			if errors.IsNotFound(err) {
				// create the resource
				reqLogger.Info("watch-keeper-limit-poll does not exist - creating")

				// set the annotation
				watchKeeperLimitPoll = *r.MakeWatchKeeperLimitPoll()
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&watchKeeperLimitPoll); err != nil {
					reqLogger.Error(err, "Failed to set annotation")
				}

				//create the resource
				err = r.client.Create(context.TODO(), &watchKeeperLimitPoll)
				if err != nil {
					reqLogger.Error(err, "Failed to create watch-keeper-limit-poll config map")
					return reconcile.Result{}, err
				}

				reqLogger.Info("watch-keeper-limit-poll config map created successfully")
				return reconcile.Result{Requeue: true}, nil
			} else {
				reqLogger.Error(err, "Failed to get watch-keeper-limit-poll config map.")
				return reconcile.Result{}, err
			}
		}
		if err == nil {
			reqLogger.Info(fmt.Sprintf("No change detected on %v", watchKeeperLimitPoll.Name))
			updatedWatchKeeperLimitPoll := r.MakeWatchKeeperLimitPoll()
			patchResult, err := patch.DefaultPatchMaker.Calculate(&watchKeeperLimitPoll, updatedWatchKeeperLimitPoll)
			if err != nil {
				reqLogger.Error(err, "Failed to calculate patch diff")
			}

			if !patchResult.IsEmpty() {
				reqLogger.Info(fmt.Sprintf("updating resource %v", watchKeeperLimitPoll.Name))
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(updatedWatchKeeperLimitPoll); err != nil {
					reqLogger.Error(err, "Failed to set annotation on ", updatedWatchKeeperLimitPoll.Name)
				}
				err = r.client.Update(context.TODO(), updatedWatchKeeperLimitPoll)
				if err != nil {
					reqLogger.Error(err, "Failed to overwrite ", updatedWatchKeeperLimitPoll.Name)
					return reconcile.Result{}, err
				}
			}

			reqLogger.Info(fmt.Sprintf("No change detected on %v", watchKeeperLimitPoll.Name))

		}

		newResources = append(newResources, watchKeeperNonNamespace.Name)
	
		// update status
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		instance.Status.RazeePrerequisitesCreated = newResources
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}
	
		// create razee-cluster-metadata
		razeeClusterMetaData := corev1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "razee-cluster-metadata", Namespace: "razee"}, &razeeClusterMetaData)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info("razee cluster metadata does not exist - creating")

				// set the annotation
				razeeClusterMetaData = *r.MakeRazeeClusterMetaData(instance)
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&razeeClusterMetaData); err != nil {
					reqLogger.Error(err, "Failed to set annotation")
				}

				err = r.client.Create(context.TODO(), &razeeClusterMetaData)
				if err != nil {
					reqLogger.Error(err, "Failed to create resource ", razeeClusterMetaData.Name)
				}

				reqLogger.Info("Resource created successfully", "resource: ", razeeClusterMetaData.Name)
				return reconcile.Result{Requeue: true}, nil
			} else {
				reqLogger.Error(err, "Failed to get resource")
				return reconcile.Result{}, err
			}
		}
		if err == nil {
			// if exists already then, overwrite
			reqLogger.Info(fmt.Sprintf("Resource already exists %v", razeeClusterMetaData.Name))

			// proposed change
			updatedRazeeClusterMetaData := *r.MakeRazeeClusterMetaData(instance)
			patchResult, err := patch.DefaultPatchMaker.Calculate(&razeeClusterMetaData, &updatedRazeeClusterMetaData)
			if err != nil {
				// handle the error
				reqLogger.Error(err, "Failed to compare patches")
			}

			if !patchResult.IsEmpty() {
				reqLogger.Info(fmt.Sprintf("Change detected on %v", razeeClusterMetaData.Name))
				// reqLogger.Info(fmt.Sprintf("Change detected on %v",razeeClusterMetaData.Name))
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&updatedRazeeClusterMetaData); err != nil {

				}
				if err != nil {
					reqLogger.Error(err, "Failed to set annotation")
					return reconcile.Result{}, err
				}
				reqLogger.Info("Updating razee-cluster-metadata")
				err = r.client.Update(context.TODO(), &updatedRazeeClusterMetaData)
				if err != nil {
					reqLogger.Error(err, "Failed to overwrite Updating razee-cluster-metadata")
					return reconcile.Result{}, err
				}
			}

			reqLogger.Info(fmt.Sprintf("No change detected on %v", razeeClusterMetaData.Name))
		}

		newResources = append(newResources,razeeClusterMetaData.Name)
	
		// update status
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		instance.Status.RazeePrerequisitesCreated = newResources
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}
	
		// create watch-keeper-config
		watchKeeperConfig := corev1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-config", Namespace: "razee"}, &watchKeeperConfig)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info("watch-keeper-config does not exist - creating")

				// set the annotation
				watchKeeperConfig = *r.MakeWatchKeeperConfig(instance)
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&watchKeeperConfig); err != nil {
					reqLogger.Error(err, "Failed to set annotation")
				}

				err = r.client.Create(context.TODO(), &watchKeeperConfig)
				if err != nil {
					reqLogger.Error(err, "Failed to create watch-keeper-config")
				}
				reqLogger.Info("watch-keeper-config created successfully")
				return reconcile.Result{Requeue: true}, nil
			} else {
				reqLogger.Error(err, "Failed to get watch-keeper-config.")
				return reconcile.Result{}, err
			}
		}
		if err == nil {
			reqLogger.Info(fmt.Sprintf("Resource already exists %v", watchKeeperConfig.Name))

			updatedWatchKeeperConfig := *r.MakeWatchKeeperConfig(instance)
			patchResult, err := patch.DefaultPatchMaker.Calculate(&watchKeeperConfig, &updatedWatchKeeperConfig)
			if err != nil {
				// handle the error
				reqLogger.Error(err, "Failed to compare patches")
			}

			if !patchResult.IsEmpty() {
				reqLogger.Info(fmt.Sprintf("Change detected on %v", watchKeeperConfig.Name))
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&updatedWatchKeeperConfig); err != nil {

				}
				reqLogger.Info(fmt.Sprintf("Updating %v", watchKeeperConfig.Name))
				err = r.client.Update(context.TODO(), &updatedWatchKeeperConfig)
				if err != nil {
					reqLogger.Error(err, "Failed to overwrite Updating razee-cluster-metadata")
					return reconcile.Result{}, err
				}
				reqLogger.Info("Updated successfully")
			}

			reqLogger.Info(fmt.Sprintf("No change detected %v", watchKeeperConfig.Name))
		}

		newResources = append(newResources, watchKeeperConfig.Name)

		// update status
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		instance.Status.RazeePrerequisitesCreated = newResources
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}
	
		// create watch-keeper-secret
		watchKeeperSecret := corev1.Secret{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-secret", Namespace: "razee"}, &watchKeeperSecret)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info("watch-keeper-secret does not exist - creating")
				watchKeeperSecret = *r.MakeWatchKeeperSecret(instance, request)
				// set the annotation
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&watchKeeperSecret); err != nil {
					reqLogger.Error(err, "Failed to set annotation")
				}
				err = r.client.Create(context.TODO(), &watchKeeperSecret)
				if err != nil {
					reqLogger.Error(err, "Failed to create watch-keeper-secret")
				}
				reqLogger.Info("watch-keeper-secret created successfully")
				return reconcile.Result{Requeue: true}, nil
			} else {
				reqLogger.Error(err, "Failed to get watch-keeper-secret.")
				return reconcile.Result{}, err
			}
		}
		if err == nil {
			reqLogger.Info(fmt.Sprintf("Resource already exists %v", watchKeeperSecret.Name))

			updatedWatchKeeperSecret := *r.MakeWatchKeeperSecret(instance, request)
			patchResult, err := patch.DefaultPatchMaker.Calculate(&watchKeeperSecret, &updatedWatchKeeperSecret)
			if err != nil {
				// handle the error
				reqLogger.Error(err, "Failed to compare patches")
			}

			if !patchResult.IsEmpty() {
				reqLogger.Info(fmt.Sprintf("Chnage detected on %v", watchKeeperSecret.Name))
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&updatedWatchKeeperSecret); err != nil {

				}
				reqLogger.Info("Updating razee-cluster-metadata")
				err = r.client.Update(context.TODO(), &updatedWatchKeeperSecret)
				if err != nil {
					reqLogger.Error(err, "Failed to overwrite Updating razee-cluster-metadata")
					return reconcile.Result{}, err
				}
				reqLogger.Info("Updated successfully")
			}

			reqLogger.Info(fmt.Sprintf("No change detected on %v", watchKeeperSecret.Name))
		}

		newResources = append(newResources,watchKeeperSecret.Name)

		// update status
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		instance.Status.RazeePrerequisitesCreated = newResources
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}
	
		// create watch-keeper-config
		ibmCosReaderKey := corev1.Secret{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "ibm-cos-reader-key", Namespace: "razee"}, &ibmCosReaderKey)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info("ibm-cos-reader-key does not exist - creating")
				ibmCosReaderKey = *r.MakeCOSReaderSecret(instance, request)
				// set the annotation
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&ibmCosReaderKey); err != nil {
					reqLogger.Error(err, "Failed to set annotation")
				}

				err = r.client.Create(context.TODO(), &ibmCosReaderKey)
				if err != nil {
					reqLogger.Error(err, "Failed to create ibm-cos-reader-key")
				}
				reqLogger.Info("ibm-cos-reader-key created successfully")
				return reconcile.Result{Requeue: true}, nil
			} else {
				reqLogger.Error(err, "Failed to get ibm-cos-reader-key.")
				return reconcile.Result{}, err
			}
		}
		if err == nil {
			reqLogger.Info(fmt.Sprintf("Resource already exists %v", ibmCosReaderKey.Name))

			updatedibmCosReaderKey := *r.MakeCOSReaderSecret(instance, request)
			patchResult, err := patch.DefaultPatchMaker.Calculate(&ibmCosReaderKey, &updatedibmCosReaderKey)
			if err != nil {
				// handle the error
				reqLogger.Error(err, "Failed to compare patches")
			}

			if !patchResult.IsEmpty() {
				reqLogger.Info(fmt.Sprintf("Change detected on %v", ibmCosReaderKey.Name))
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&updatedibmCosReaderKey); err != nil {

				}
				reqLogger.Info("Updating ribm-cos-reader-key")
				err = r.client.Update(context.TODO(), &updatedibmCosReaderKey)
				if err != nil {
					reqLogger.Error(err, "Failed to overwrite ibm-cos-reader-key")
					return reconcile.Result{}, err
				}
				reqLogger.Info("Updated successfully")
			}

			reqLogger.Info(fmt.Sprintf("No change detected on %v", ibmCosReaderKey.Name))
		}

		newResources = append(newResources, ibmCosReaderKey.Name)
	
		// update status
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		instance.Status.RazeePrerequisitesCreated = newResources
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}

		/******************************************************************************
		CREATE THE RAZEE JOB
		/******************************************************************************/
		if instance.Status.JobState.Succeeded != 1 {
			job := r.MakeRazeeJob(request, instance)

			// Check if the Job exists already
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      RAZEE_DEPLOY_JOB,
					Namespace: request.Namespace,
				},
			}

			foundJob := batch.Job{}
			err = r.client.Get(context.TODO(), req.NamespacedName, &foundJob)
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

			if err := controllerutil.SetControllerReference(instance, &foundJob, r.scheme); err != nil {
				reqLogger.Error(err, "Failed to set controller reference")
				return reconcile.Result{}, err
			}
		}

		// if the job succeeds apply the parentRRS3 and patch resources, add "parentRRS3" to
		if instance.Status.JobState.Succeeded == 1 {
			parentRRS3 := &unstructured.Unstructured{}
			parentRRS3.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "deploy.razee.io",
				Kind:    "RemoteResourceS3",
				Version: "v1alpha2",
			})
			err = r.client.Get(context.TODO(), client.ObjectKey{Name: "parent", Namespace: "razee"}, parentRRS3)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info("parent RRS3 does not exist - creating")
					// set the annotation
					parentRRS3 = r.MakeParentRemoteResourceS3(instance)
					if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(parentRRS3); err != nil {
						reqLogger.Error(err, "Failed to set annotation")
					}
					err = r.client.Create(context.TODO(), parentRRS3)
					if err != nil {
						reqLogger.Error(err, "Failed to create parent RRS3")
						return reconcile.Result{}, err
					}

					reqLogger.Info("parent RRS3 created successfully")
				} else {
					reqLogger.Error(err, "Failed to get parent RRS3.")
					return reconcile.Result{}, err
				}
			}
			if err == nil {
				reqLogger.Info("parent RRS3 already exists")

				//TODO: could functionalize this
				updatedParentRRS3 := r.MakeParentRemoteResourceS3(instance)
				updatedParentRRS3.SetAnnotations(parentRRS3.GetAnnotations())
				updatedParentRRS3.SetCreationTimestamp(parentRRS3.GetCreationTimestamp())
				updatedParentRRS3.SetFinalizers(parentRRS3.GetFinalizers())
				updatedParentRRS3.SetGeneration(parentRRS3.GetGeneration())
				updatedParentRRS3.SetResourceVersion(parentRRS3.GetResourceVersion())
				updatedParentRRS3.SetSelfLink(parentRRS3.GetSelfLink())
				updatedParentRRS3.SetUID(parentRRS3.GetUID())

				patchResult, err := patch.DefaultPatchMaker.Calculate(parentRRS3,updatedParentRRS3,patch.IgnoreStatusFields())
				if err != nil {
					// handle the error
					reqLogger.Error(err, "Failed to compare patches")
				}

				if !patchResult.IsEmpty() {
					reqLogger.Info(fmt.Sprintf("Change detected on %v", updatedParentRRS3.GetName()))

					// just updating the url for now
					parentRRS3.Object["spec"] = updatedParentRRS3.Object["spec"]

					if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(parentRRS3); err != nil {
						reqLogger.Error(err, "Failed to set annotation")
					}
					reqLogger.Info("Updating parentRRS3")
					err = r.client.Update(context.TODO(), parentRRS3)
					if err != nil {
						reqLogger.Error(err, "Failed to overwrite parentRRS3")
						return reconcile.Result{}, err
					}
					reqLogger.Info("Updated successfully")
				}

				reqLogger.Info(fmt.Sprintf("No change detected on %v", updatedParentRRS3.GetName()))
			}

			newResources = append(newResources, "parentRRS3")
			/******************************************************************************
			PATCH RESOURCES FOR DIANEMO
			Patch the Console and Infrastructure resources with the watch-keeper label
			Patch 'razee-cluster-metadata' and add data.name: "max-test-uuid"
			/******************************************************************************/
			reqLogger.Info("finding Console resource")
			console := &unstructured.Unstructured{}
			console.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "config.openshift.io",
				Kind:    "Console",
				Version: "v1",
			})
			err = r.client.Get(context.Background(), client.ObjectKey{
				Name: "cluster",
			}, console)
			if err != nil {
				reqLogger.Error(err, "Failed to retrieve Console resource")
				return reconcile.Result{}, err
			}

			reqLogger.Info("Found Console resource")
			//TODO: check labels for razee/watch-resource:lite
			consoleLabels := console.GetLabels()
			if !reflect.DeepEqual(consoleLabels,map[string]string{"razee/watch-resource": "lite"}) || consoleLabels == nil {
				console.SetLabels(map[string]string{"razee/watch-resource": "lite"})
				err = r.client.Update(context.TODO(), console)
				if err != nil {
					reqLogger.Error(err, "Failed to patch Console resource")
					return reconcile.Result{}, err
				}
				reqLogger.Info("Patched Console resource")
			}
			reqLogger.Info("No patch needed on Console resource")

			// Patch the Infrastructure resource
			reqLogger.Info("finding Infrastructure resource")
			infrastructureResource := &unstructured.Unstructured{}
			infrastructureResource.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "config.openshift.io",
				Kind:    "Infrastructure",
				Version: "v1",
			})
			err = r.client.Get(context.Background(), client.ObjectKey{
				Name: "cluster",
			}, infrastructureResource)
			if err != nil {
				reqLogger.Error(err, "Failed to retrieve Infrastructure resource")
				return reconcile.Result{}, err
			}

			reqLogger.Info("Found Infrastructure resource")
			infrastructureLabels := infrastructureResource.GetLabels()
			if !reflect.DeepEqual(infrastructureLabels,map[string]string{"razee/watch-resource": "lite"}) || infrastructureLabels == nil {
				infrastructureResource.SetLabels(map[string]string{"razee/watch-resource": "lite"})
				err = r.client.Update(context.TODO(), infrastructureResource)
				if err != nil {
					reqLogger.Error(err, "Failed to patch Infrastructure resource")
					return reconcile.Result{}, err
				}
				reqLogger.Info("Patched Infrastructure resource")
			}
			reqLogger.Info("No patch needed on Infrastructure resource")

		}

		// update status
		reqLogger.Info("updating Status.RazeePrerequisitesCreated")
		instance.Status.RazeePrerequisitesCreated = newResources
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}

	}
	// reqLogger.Info("End of reconcile")
	return reconcile.Result{}, nil

}

func (r *ReconcileRazeeDeployment) reconcileRhmOperatorSecret(request *reconcile.Request) (*reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "request.Name", request.Name)
	reqLogger.Info("Beginning of rhm-operator-secret reconcile")

	// retrieve the razee instance
	razeeDeployments := &marketplacev1alpha1.RazeeDeploymentList{}
	err := r.client.List(context.TODO(), razeeDeployments)
	if err != nil {
		reqLogger.Error(err, "Failed to list RazeeDeployments")
		return &reconcile.Result{}, err
	}

	razeeInstance := razeeDeployments.Items[0]

	// get the operator secret
	rhmOperatorSecret := corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      RHM_OPERATOR_SECRET_NAME,
		Namespace: request.Namespace,
	}, &rhmOperatorSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to find operator secret")
			return nil, nil
		}
		return nil, err
	}

	/******************************************************************************
	UPDATE SPEC.DEPLOYSECRETVALUES
	/******************************************************************************/

	// set non-nil pointer
	razeeConfigurationValues := marketplacev1alpha1.RazeeConfigurationValues{}
	razeeInstance.Spec.DeployConfig = &razeeConfigurationValues

	razeeConfigurationValues, missingItems, err := utils.ConvertSecretToStruct(rhmOperatorSecret.Data)

	razeeInstance.Status.MissingDeploySecretValues = missingItems
	razeeInstance.Spec.DeployConfig = &razeeConfigurationValues

	reqLogger.Info("Updating spec with missing items and secret values")
	err = r.client.Update(context.TODO(), &razeeInstance)
	if err != nil {
		reqLogger.Error(err, "Failed to update Spec.DeploySecretValues")
		return &reconcile.Result{}, err
	}

	reqLogger.Info("End of rhm-operator-secret reconcile")
	return nil, nil
}

// finalizeRazeeDeployment cleans up resources before the RazeeDeployment CR is deleted
func (r *ReconcileRazeeDeployment) finalizeRazeeDeployment(req *marketplacev1alpha1.RazeeDeployment) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("running finalizer")

	jobName := types.NamespacedName{
		Name:      "razeedeploy-job",
		Namespace: req.Namespace,
	}

	foundJob := batch.Job{}
	reqLogger.Info("finding install job")
	err := r.client.Get(context.TODO(), jobName, &foundJob)
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
	}

	if !errors.IsNotFound(err) {
		reqLogger.Info("cleaning up install job")
		err := r.client.Delete(context.TODO(), &foundJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "cleaning up install job")
			return reconcile.Result{}, err
		}
	} else {
		reqLogger.Info("found no job to clean up")
	}

	// Deploy a job to delete razee if we need to
	if req.Status.RazeeJobInstall != nil {
		jobName.Name = RAZEE_UNINSTALL_NAME
		foundJob = batch.Job{}
		reqLogger.Info("razee was installed; finding uninstall job")
		err = r.client.Get(context.TODO(), jobName, &foundJob)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating razee-uninstall-job")
			job := r.MakeRazeeUninstallJob(req.Namespace, req.Status.RazeeJobInstall)
			err = r.client.Create(context.TODO(), job)
			if err != nil {
				reqLogger.Error(err, "Failed to create Job on cluster")
				return reconcile.Result{}, err
			}
			reqLogger.Info("job created successfully")
			return reconcile.Result{RequeueAfter: time.Second * 5}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get Job(s) from Cluster")
			return reconcile.Result{}, err
		}

		reqLogger.Info("found uninstall job")

		if len(foundJob.Status.Conditions) == 0 {
			reqLogger.Info("RazeeUninstallJob Conditions have not been propagated yet")
			return reconcile.Result{RequeueAfter: time.Second * 30}, nil
		}

		if foundJob.Status.Succeeded < 1 && foundJob.Status.Failed <= 3 {
			reqLogger.Info("RazeeUnisntallJob is not successful")
			return reconcile.Result{RequeueAfter: time.Second * 30}, nil
		}

		reqLogger.Info("Deleteing uninstall job")
		err = r.client.Delete(context.TODO(), &foundJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil {
			if !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
		}
	}

	reqLogger.Info("Uninstall job created successfully")
	reqLogger.Info("Successfully finalized RazeeDeployment")

	// Remove the razeeDeploymentFinalizer
	// Once all finalizers are removed, the object will be deleted
	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), razeeDeploymentFinalizer))
	err = r.client.Update(context.TODO(), req)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// MakeRazeeJob returns a Batch.Job which installs razee
func (r *ReconcileRazeeDeployment) MakeRazeeJob(request reconcile.Request, instance *marketplacev1alpha1.RazeeDeployment) *batch.Job {
	image := viper.GetString("razee-job-image")
	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "razeedeploy-job",
			Namespace: request.Namespace,
		},
		Spec: batch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "redhat-marketplace-operator",
					Containers: []corev1.Container{{
						Name:    "razeedeploy-job",
						Image:   image,
						Command: []string{"node", "src/install", "--namespace=razee"},
						Args:    []string{fmt.Sprintf("--file-source=%v", instance.Spec.DeployConfig.FileSourceURL), "--autoupdate"},
					}},
					RestartPolicy: "Never",
				},
			},
		},
	}
}

// MakeRazeeUninstalllJob returns a Batch.Job which uninstalls razee
func (r *ReconcileRazeeDeployment) MakeRazeeUninstallJob(namespace string, razeeJob *marketplacev1alpha1.RazeeJobInstallStruct) *batch.Job {
	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RAZEE_UNINSTALL_NAME,
			Namespace: namespace,
		},
		Spec: batch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: utils.RAZEE_SERVICE_ACCOUNT,
					Containers: []corev1.Container{{
						Name:    RAZEE_UNINSTALL_NAME,
						Image:   r.opts.RazeeJobImage,
						Command: []string{"node", "src/remove", fmt.Sprintf("--namespace=%s", razeeJob.RazeeNamespace)},
						Args:    []string{fmt.Sprintf("--file-source=%v", razeeJob.RazeeInstallURL), "--autoupdate"},
					}},
					RestartPolicy: "Never",
				},
			},
		},
	}
}

// addFinalizer adds finalizers to the RazeeDeployment CR
func (r *ReconcileRazeeDeployment) addFinalizer(razee *marketplacev1alpha1.RazeeDeployment, namespace string) error {
	reqLogger := log.WithValues("Request.Namespace", namespace, "Request.Name", RAZEE_UNINSTALL_NAME)
	reqLogger.Info("Adding Finalizer for the razeeDeploymentFinzliaer")
	razee.SetFinalizers(append(razee.GetFinalizers(), razeeDeploymentFinalizer))

	err := r.client.Update(context.TODO(), razee)
	if err != nil {
		reqLogger.Error(err, "Failed to update RazeeDeployment with the Finalizer")
		return err
	}
	return nil
}

func (r *ReconcileRazeeDeployment) MakeRazeeClusterMetaData(instance *marketplacev1alpha1.RazeeDeployment) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "razee-cluster-metadata",
			Namespace: RAZEE_NAMESPACE,
			Labels: map[string]string{
				"razee/cluster-metadata": "true",
				"razee/watch-resource":   "lite",
			},
		},
		Data: map[string]string{"name": instance.Spec.ClusterUUID},
	}
}

//watch-keeper-non-namespace
func (r *ReconcileRazeeDeployment) MakeWatchKeeperNonNamespace() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-non-namespaced",
			Namespace: RAZEE_NAMESPACE,
		},
		Data: map[string]string{"v1_namespace": "true"},
	}
}

//watch-keeper-non-namespace
func (r *ReconcileRazeeDeployment) MakeWatchKeeperLimitPoll() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-limit-poll",
			Namespace: RAZEE_NAMESPACE,
		},
	}
}

//DeploySecretValues[RAZEE_DASH_URL_FIELD]
func (r *ReconcileRazeeDeployment) MakeWatchKeeperConfig(instance *marketplacev1alpha1.RazeeDeployment) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-config",
			Namespace: RAZEE_NAMESPACE,
		},
		Data: map[string]string{"RAZEEDASH_URL": instance.Spec.DeployConfig.RazeeDashUrl, "START_DELAY_MAX": "0"},
	}
}

// DeploySecretValues[RAZEE_DASH_ORG_KEY_FIELD]
func (r *ReconcileRazeeDeployment) GetDataFromRhmSecret(request reconcile.Request, sel corev1.SecretKeySelector) (*reconcile.Result, error, []byte) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "request.Name", request.Name)
	reqLogger.Info("Beginning of rhm-operator-secret reconcile")
	// get the operator secret
	rhmOperatorSecret := corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      RHM_OPERATOR_SECRET_NAME,
		Namespace: request.Namespace,
	}, &rhmOperatorSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to find operator secret")
			return nil, nil, nil
		}
		return nil, err, nil
	}
	key, err := utils.ExtractCredKey(&rhmOperatorSecret, sel)
	return nil, err, key
}

func (r *ReconcileRazeeDeployment) MakeWatchKeeperSecret(instance *marketplacev1alpha1.RazeeDeployment, request reconcile.Request) *corev1.Secret {
	selector := instance.Spec.DeployConfig.RazeeDashOrgKey
	//TODO: fill out with unused vars
	_, _, key := r.GetDataFromRhmSecret(request, *selector)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-secret",
			Namespace: RAZEE_NAMESPACE,
		},
		Data: map[string][]byte{"RAZEEDASH_ORG_KEY": key},
	}
}

func (r *ReconcileRazeeDeployment) MakeCOSReaderSecret(instance *marketplacev1alpha1.RazeeDeployment, request reconcile.Request) *corev1.Secret {
	selector := instance.Spec.DeployConfig.IbmCosReaderKey
	_, _, key := r.GetDataFromRhmSecret(request, *selector)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ibm-cos-reader-key",
			Namespace: RAZEE_NAMESPACE,
		},
		Data: map[string][]byte{"accesskey": []byte(key)},
	}
}

func (r *ReconcileRazeeDeployment) MakeParentRemoteResourceS3(instance *marketplacev1alpha1.RazeeDeployment) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deploy.razee.io/v1alpha2",
			"kind":       "RemoteResourceS3",
			"metadata": map[string]interface{}{
				"name":      "parent",
				"namespace": RAZEE_NAMESPACE,
			},
			"spec": map[string]interface{}{
				"auth": map[string]interface{}{
					"iam": map[string]interface{}{
						"response_type": "cloud_iam",
						"url":           `https://iam.cloud.ibm.com/identity/token`,
						"grant_type":    "urn:ibm:params:oauth:grant-type:apikey",
						"api_key": map[string]interface{}{
							"valueFrom": map[string]interface{}{
								"secretKeyRef": map[string]interface{}{
									"name": "ibm-cos-reader-key",
									"key":  "accesskey",
								},
							},
						},
					},
				},
				"requests": []interface{}{
					map[string]map[string]string{"options": {"url": *instance.Spec.ChildUrl}},
				},
			},
		},
	}
}
