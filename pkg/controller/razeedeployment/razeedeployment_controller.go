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
	appsv1 "k8s.io/api/apps/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	RAZEE_DEPLOYMENT_FINALIZER = "razeedeploy.finalizer.marketplace.redhat.com"
	COS_READER_KEY_NAME        = "rhm-cos-reader-key"
	RAZEE_UNINSTALL_NAME       = "razee-uninstall-job"
	DEFAULT_RAZEE_JOB_IMAGE    = "quay.io/razee/razeedeploy-delta:1.1.0"
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

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.RazeeDeployment{},
	})
	if err != nil {
		return err
	}

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
	opts   *RazeeOpts
}

type RazeeOpts struct {
	RazeeJobImage string
	ClusterUUID   string
}

// Reconcile reads that state of the cluster for a RazeeDeployment object and makes changes based on the state read
// and what is in the RazeeDeployment.Spec
func (r *ReconcileRazeeDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RazeeDeployment")
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
	if !utils.Contains(instance.GetFinalizers(), RAZEE_DEPLOYMENT_FINALIZER) {
		if err := r.addFinalizer(instance, request.Namespace); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Check if the RazeeDeployment instance is being marked for deletion
	isMarkedForDeletion := instance.GetDeletionTimestamp() != nil
	if isMarkedForDeletion {
		if utils.Contains(instance.GetFinalizers(), RAZEE_DEPLOYMENT_FINALIZER) {
			//Run finalization logic for the RAZEE_DEPLOYMENT_FINALIZER.
			//If it fails, don't remove the finalizer so we can retry during the next reconcile
			return r.finalizeRazeeDeployment(instance)
		}
		return reconcile.Result{}, nil
	}

	if instance.Spec.TargetNamespace == nil {
		if instance.Status.RazeeJobInstall != nil {
			instance.Spec.TargetNamespace = &instance.Status.RazeeJobInstall.RazeeNamespace
		} else {
			instance.Spec.TargetNamespace = &instance.Namespace
		}
		err := r.client.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}

		reqLogger.Info("set target namespace to", "namespace", instance.Spec.TargetNamespace)
		return reconcile.Result{}, nil
	}

	/******************************************************************************
	PROCEED WITH CREATING RAZEE PREREQUISITES? YES/NO
	do we have all the fields from rhm-secret ? (combined secret)
	check that we can continue with applying the razee job
	if the job has already run exit
	if there are still missing resources exit
	/******************************************************************************/
	//TODO: can I combine this logic ?
	if instance.Spec.DeployConfig == nil {
		reqLogger.Info("rhm-operator-secret has not been applied")
		return r.reconcileRhmOperatorSecret(*instance, request)
	}

	if instance.Spec.DeployConfig != nil {
		if len(instance.Status.MissingDeploySecretValues) > 0 {
			reqLogger.Info("Missing required razee configuration values")
			return r.reconcileRhmOperatorSecret(*instance, request)
		} else {
			reqLogger.Info("all secret values found")
			//construct the childURL
			url := fmt.Sprintf("%s/%s/%s/%s", instance.Spec.DeployConfig.IbmCosURL, instance.Spec.DeployConfig.BucketName, instance.Spec.ClusterUUID, instance.Spec.DeployConfig.ChildRSSFIleName)
			instance.Spec.ChildUrl = &url
			err = r.client.Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update ChildUrl")
				return reconcile.Result{}, err
			}

			// Update the Spec TargetNamespace
			reqLogger.Info("All required razee configuration values have been found")
		}

	}

	/******************************************************************************
	APPLY OR OVERWRITE RAZEE RESOURCES
	/******************************************************************************/
	newResources := []string{}
	razeeNamespace := corev1.Namespace{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: *instance.Spec.TargetNamespace}, &razeeNamespace)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("razee namespace does not exist - creating")
			razeeNamespace.ObjectMeta.Name = *instance.Spec.TargetNamespace
			err = r.client.Create(context.TODO(), &razeeNamespace)
			if err != nil {
				reqLogger.Error(err, "Failed to create razee namespace.")
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		} else {
			reqLogger.Error(err, "Failed to get razee ns.")
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.Info("razee namespace already exists")
	}

	newResources = append(newResources, fmt.Sprintf("%v namespace", razeeNamespace.Name))

	// update status
	reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
	instance.Status.RazeePrerequisitesCreated = newResources
	err = r.client.Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update status")
		return reconcile.Result{}, err
	}

	watchKeeperNonNamespace := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-non-namespaced", Namespace: *instance.Spec.TargetNamespace}, &watchKeeperNonNamespace)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watch-keeper-non-namespace does not exist - creating")

			// set the annotation
			watchKeeperNonNamespace = *r.MakeWatchKeeperNonNamespace(instance)
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&watchKeeperNonNamespace); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
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

		updatedWatchKeeperNonNameSpace := r.MakeWatchKeeperNonNamespace(instance)
		patchResult, err := patch.DefaultPatchMaker.Calculate(&watchKeeperNonNamespace, updatedWatchKeeperNonNameSpace)
		if err != nil {
			reqLogger.Error(err, "Failed to compare patches")
			return reconcile.Result{}, err
		}

		if !patchResult.IsEmpty() {
			reqLogger.Info(fmt.Sprintf("Change detected on %v", watchKeeperNonNamespace.Name))
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(updatedWatchKeeperNonNameSpace); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			reqLogger.Info("Updating resource", "resource: ", watchKeeperNonNamespace.Name)
			err = r.client.Update(context.TODO(), updatedWatchKeeperNonNameSpace)
			if err != nil {
				reqLogger.Info(fmt.Sprintf("Failed to update %v", watchKeeperNonNamespace.Name))
				return reconcile.Result{}, err
			}
		}

		reqLogger.Info(fmt.Sprintf("No change detected on %v", watchKeeperNonNamespace.Name))

	}

	newResources = append(newResources, watchKeeperNonNamespace.Name)

	// update status
	reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
	instance.Status.RazeePrerequisitesCreated = newResources
	err = r.client.Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update status")
	}

	// apply watch-keeper-limit-poll config map
	watchKeeperLimitPoll := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-limit-poll", Namespace: *instance.Spec.TargetNamespace}, &watchKeeperLimitPoll)
	if err != nil {
		if errors.IsNotFound(err) {
			// create the resource
			reqLogger.Info("watch-keeper-limit-poll does not exist - creating")

			watchKeeperLimitPoll = *r.MakeWatchKeeperLimitPoll(instance)
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&watchKeeperLimitPoll); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

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
		updatedWatchKeeperLimitPoll := r.MakeWatchKeeperLimitPoll(instance)
		patchResult, err := patch.DefaultPatchMaker.Calculate(&watchKeeperLimitPoll, updatedWatchKeeperLimitPoll)
		if err != nil {
			reqLogger.Error(err, "Failed to calculate patch diff")
			return reconcile.Result{}, err
		}

		if !patchResult.IsEmpty() {
			reqLogger.Info(fmt.Sprintf("updating resource %v", watchKeeperLimitPoll.Name))
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(updatedWatchKeeperLimitPoll); err != nil {
				reqLogger.Error(err, "Failed to set annotation on ", updatedWatchKeeperLimitPoll.Name)
				return reconcile.Result{}, err
			}
			err = r.client.Update(context.TODO(), updatedWatchKeeperLimitPoll)
			if err != nil {
				reqLogger.Error(err, "Failed to overwrite ", updatedWatchKeeperLimitPoll.Name)
				return reconcile.Result{}, err
			}
		}

		reqLogger.Info(fmt.Sprintf("No change detected on %v", watchKeeperLimitPoll.Name))

	}

	newResources = append(newResources, watchKeeperLimitPoll.Name)

	// update status
	reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
	instance.Status.RazeePrerequisitesCreated = newResources
	err = r.client.Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update status")
	}

	// create razee-cluster-metadata
	razeeClusterMetaData := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "razee-cluster-metadata", Namespace: *instance.Spec.TargetNamespace}, &razeeClusterMetaData)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("razee cluster metadata does not exist - creating")

			razeeClusterMetaData = *r.MakeRazeeClusterMetaData(instance)
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&razeeClusterMetaData); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			err = r.client.Create(context.TODO(), &razeeClusterMetaData)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource ", razeeClusterMetaData.Name)
				return reconcile.Result{}, err
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

		updatedRazeeClusterMetaData := *r.MakeRazeeClusterMetaData(instance)
		patchResult, err := patch.DefaultPatchMaker.Calculate(&razeeClusterMetaData, &updatedRazeeClusterMetaData)
		if err != nil {
			reqLogger.Error(err, "Failed to compare patches")
			return reconcile.Result{}, err
		}

		if !patchResult.IsEmpty() {
			reqLogger.Info(fmt.Sprintf("Change detected on %v", razeeClusterMetaData.Name))
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&updatedRazeeClusterMetaData); err != nil {
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

	newResources = append(newResources, razeeClusterMetaData.Name)

	// update status
	reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
	instance.Status.RazeePrerequisitesCreated = newResources
	err = r.client.Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update status")
	}

	// create watch-keeper-config
	watchKeeperConfig := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-config", Namespace: *instance.Spec.TargetNamespace}, &watchKeeperConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watch-keeper-config does not exist - creating")

			watchKeeperConfig = *r.MakeWatchKeeperConfig(instance)
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&watchKeeperConfig); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			err = r.client.Create(context.TODO(), &watchKeeperConfig)
			if err != nil {
				reqLogger.Error(err, "Failed to create watch-keeper-config")
				return reconcile.Result{}, err
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
			reqLogger.Error(err, "Failed to compare patches")
		}

		if !patchResult.IsEmpty() {
			reqLogger.Info(fmt.Sprintf("Change detected on %v", watchKeeperConfig.Name))
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&updatedWatchKeeperConfig); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
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
		return reconcile.Result{}, err
	}

	// create watch-keeper-secret
	watchKeeperSecret := corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-secret", Namespace: *instance.Spec.TargetNamespace}, &watchKeeperSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watch-keeper-secret does not exist - creating")
			watchKeeperSecret = *r.MakeWatchKeeperSecret(instance, request)

			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&watchKeeperSecret); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}
			err = r.client.Create(context.TODO(), &watchKeeperSecret)
			if err != nil {
				reqLogger.Error(err, "Failed to create watch-keeper-secret")
				return reconcile.Result{}, err
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
			reqLogger.Error(err, "Failed to compare patches")
			return reconcile.Result{}, err
		}

		if !patchResult.IsEmpty() {
			reqLogger.Info(fmt.Sprintf("Chnage detected on %v", watchKeeperSecret.Name))
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&updatedWatchKeeperSecret); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
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

	newResources = append(newResources, watchKeeperSecret.Name)

	// update status
	reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
	instance.Status.RazeePrerequisitesCreated = newResources
	err = r.client.Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update status")
		return reconcile.Result{}, err
	}

	// create cos-reader-key-secret
	ibmCosReaderKey := corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: COS_READER_KEY_NAME, Namespace: *instance.Spec.TargetNamespace}, &ibmCosReaderKey)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("ibm-cos-reader-key does not exist - creating")
			ibmCosReaderKey, err = r.MakeCOSReaderSecret(instance, request)

			if err = patch.DefaultAnnotator.SetLastAppliedAnnotation(&ibmCosReaderKey); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			err = r.client.Create(context.TODO(), &ibmCosReaderKey)
			if err != nil {
				reqLogger.Error(err, "Failed to create ibm-cos-reader-key")
				return reconcile.Result{}, err
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

		updatedibmCosReaderKey, err := r.MakeCOSReaderSecret(instance, request)
		patchResult, err := patch.DefaultPatchMaker.Calculate(&ibmCosReaderKey, &updatedibmCosReaderKey)
		if err != nil {
			reqLogger.Error(err, "Failed to compare patches")
			return reconcile.Result{}, err
		}

		if !patchResult.IsEmpty() {
			reqLogger.Info(fmt.Sprintf("Change detected on %v", ibmCosReaderKey.Name))
			if err = patch.DefaultAnnotator.SetLastAppliedAnnotation(&updatedibmCosReaderKey); err != nil {
				reqLogger.Info("Failed to set annotation")
				return reconcile.Result{}, err
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
		return reconcile.Result{}, err
	}

	/******************************************************************************
	CREATE THE RAZEE JOB
	/******************************************************************************/
	// if the job hasn't succeeded create it
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
		if err != nil {
			if errors.IsNotFound(err) {
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
				//TODO: is the requeue necessary here
				return reconcile.Result{RequeueAfter: time.Second * 30}, nil
				// return reconcile.Result{Requeue: true}, nil
				// return reconcile.Result{}, nil
			} else {
				reqLogger.Error(err, "Failed to get Job.")
				return reconcile.Result{}, err
			}
		} 

		if err := controllerutil.SetControllerReference(instance, &foundJob, r.scheme); err != nil {
			reqLogger.Error(err, "Failed to set controller reference")
			return reconcile.Result{}, err
		}

		// return r.reconcileRazeeDeployJob(*instance, request)
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

			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update JobState")
				return reconcile.Result{}, err
			}
			reqLogger.Info("Updated JobState")
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

		reqLogger.Info("End of razee job reconciler")
	}

	// if the job succeeds apply the parentRRS3 and patch resources, add "parentRRS3" to
	if instance.Status.JobState.Succeeded == 1 {
		parentRRS3 := &unstructured.Unstructured{}
		parentRRS3.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "deploy.razee.io",
			Kind:    "RemoteResourceS3",
			Version: "v1alpha2",
		})
		err = r.client.Get(context.TODO(), client.ObjectKey{Name: "parent", Namespace: *instance.Spec.TargetNamespace}, parentRRS3)
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

			updatedParentRRS3 := r.MakeParentRemoteResourceS3(instance)
			updatedParentRRS3.SetAnnotations(parentRRS3.GetAnnotations())
			updatedParentRRS3.SetCreationTimestamp(parentRRS3.GetCreationTimestamp())
			updatedParentRRS3.SetFinalizers(parentRRS3.GetFinalizers())
			updatedParentRRS3.SetGeneration(parentRRS3.GetGeneration())
			updatedParentRRS3.SetResourceVersion(parentRRS3.GetResourceVersion())
			updatedParentRRS3.SetSelfLink(parentRRS3.GetSelfLink())
			updatedParentRRS3.SetUID(parentRRS3.GetUID())

			patchResult, err := patch.DefaultPatchMaker.Calculate(parentRRS3, updatedParentRRS3, patch.IgnoreStatusFields())
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
		// update status
		reqLogger.Info("updating Status.RazeePrerequisitesCreated with parent rrs3")
		instance.Status.RazeePrerequisitesCreated = newResources
		// patch := client.MergeFrom(instance.DeepCopy())
		// err = r.client.Status().Patch(context.TODO(), instance, patch)
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}

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
		if !reflect.DeepEqual(consoleLabels, map[string]string{"razee/watch-resource": "lite"}) || consoleLabels == nil {
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
		if !reflect.DeepEqual(infrastructureLabels, map[string]string{"razee/watch-resource": "lite"}) || infrastructureLabels == nil {
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

	reqLogger.Info("End of reconcile")
	return reconcile.Result{}, nil

}

func (r *ReconcileRazeeDeployment) reconcileRazeeDeployJob(instance marketplacev1alpha1.RazeeDeployment, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Beginning of Razeedeploy-job controller Instance reconciler")
	// Fetch the RazeeDeployment instance
	// instance := &marketplacev1alpha1.RazeeDeployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      request.Name,
		Namespace: request.Namespace,
	}, &instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Error(err, "Failed to find RazeeDeployment instance")
			//TODO: is returning nil here correct ? 
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get Razee Deployment")
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

		reqLogger.Info("updating status inside job reconciler")
		err = r.client.Status().Update(context.TODO(), &instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update JobState")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Updated JobState")
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

	reqLogger.Info("End of razee job reconciler")
	//TODO: returning nil here is wrong
	return reconcile.Result{}, nil
}

func (r *ReconcileRazeeDeployment) reconcileRhmOperatorSecret(instance marketplacev1alpha1.RazeeDeployment, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "request.Name", request.Name)
	reqLogger.Info("Beginning of rhm-operator-secret reconcile")

	// get the operator secret
	rhmOperatorSecret := corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      *instance.Spec.DeploySecretName,
		Namespace: request.Namespace,
	}, &rhmOperatorSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to find operator secret")
			//TODO: set this to 60 secs
			return reconcile.Result{RequeueAfter: time.Second * 30}, nil
		} else {
			return reconcile.Result{}, err
		}
	}

	razeeConfigurationValues := marketplacev1alpha1.RazeeConfigurationValues{}
	instance.Spec.DeployConfig = &razeeConfigurationValues

	// if instance.Status.LocalSecretVarsPopulated == nil {
	// 	//TODO: is the dereferencing on the left ok here ? 
	// 	*instance.Status.LocalSecretVarsPopulated = false
	// }

	// //TODO: have a question for Zach on how to best update the spec and status
	// // Might as well use this
	// // was running into issues before
	// if instance.Status.RedHatMarketplaceSecretFound == nil {
	// 	//TODO: is the dereferencing on the left ok here ? 
	// 	*instance.Status.RedHatMarketplaceSecretFound = false
	// }

	razeeConfigurationValues, missingItems, err := utils.ConvertSecretToStruct(rhmOperatorSecret.Data)

	instance.Status.MissingDeploySecretValues = missingItems
	instance.Spec.DeployConfig = &razeeConfigurationValues

	reqLogger.Info("Updating spec with missing items and secret values")
	err = r.client.Update(context.TODO(), &instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update Spec.DeploySecretValues")
		return reconcile.Result{}, err
	}

	reqLogger.Info("End of rhm-operator-secret reconcile")
	return reconcile.Result{}, nil
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

	// Remove the RAZEE_DEPLOYMENT_FINALIZER
	// Once all finalizers are removed, the object will be deleted
	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), RAZEE_DEPLOYMENT_FINALIZER))
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
	razee.SetFinalizers(append(razee.GetFinalizers(), RAZEE_DEPLOYMENT_FINALIZER))

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
			Namespace: *instance.Spec.TargetNamespace,
			Labels: map[string]string{
				"razee/cluster-metadata": "true",
				"razee/watch-resource":   "lite",
			},
		},
		Data: map[string]string{"name": instance.Spec.ClusterUUID},
	}
}

//watch-keeper-non-namespace
func (r *ReconcileRazeeDeployment) MakeWatchKeeperNonNamespace(
	instance *marketplacev1alpha1.RazeeDeployment,
) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-non-namespaced",
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string]string{"v1_namespace": "true"},
	}
}

//watch-keeper-non-namespace
func (r *ReconcileRazeeDeployment) MakeWatchKeeperLimitPoll(
	instance *marketplacev1alpha1.RazeeDeployment,
) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-limit-poll",
			Namespace: *instance.Spec.TargetNamespace,
		},
	}
}

//DeploySecretValues[RAZEE_DASH_URL_FIELD]
func (r *ReconcileRazeeDeployment) MakeWatchKeeperConfig(instance *marketplacev1alpha1.RazeeDeployment) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-config",
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string]string{"RAZEEDASH_URL": instance.Spec.DeployConfig.RazeeDashUrl, "START_DELAY_MAX": "0"},
	}
}

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

//TODO: follow the same pattern with MakeCOSReaderKey
func (r *ReconcileRazeeDeployment) MakeWatchKeeperSecret(instance *marketplacev1alpha1.RazeeDeployment, request reconcile.Request) *corev1.Secret {
	selector := instance.Spec.DeployConfig.RazeeDashOrgKey
	_, _, key := r.GetDataFromRhmSecret(request, *selector)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-secret",
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string][]byte{"RAZEEDASH_ORG_KEY": key},
	}
}

func (r *ReconcileRazeeDeployment) MakeCOSReaderSecret(instance *marketplacev1alpha1.RazeeDeployment, request reconcile.Request) (corev1.Secret, error) {
	selector := instance.Spec.DeployConfig.IbmCosReaderKey
	_, err, key := r.GetDataFromRhmSecret(request, *selector)
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      COS_READER_KEY_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string][]byte{"accesskey": []byte(key)},
	}, err
}

func (r *ReconcileRazeeDeployment) MakeParentRemoteResourceS3(instance *marketplacev1alpha1.RazeeDeployment) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deploy.razee.io/v1alpha2",
			"kind":       "RemoteResourceS3",
			"metadata": map[string]interface{}{
				"name":      "parent",
				"namespace": *instance.Spec.TargetNamespace,
			},
			"spec": map[string]interface{}{
				"auth": map[string]interface{}{
					"iam": map[string]interface{}{
						"responseType": "cloud_iam",
						"url":          `https://iam.cloud.ibm.com/identity/token`,
						"grantType":    "urn:ibm:params:oauth:grant-type:apikey",
						"apiKeyRef": map[string]interface{}{
							"valueFrom": map[string]interface{}{
								"secretKeyRef": map[string]interface{}{
									"name": COS_READER_KEY_NAME,
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

//completeUninstall() deploys a batch.job which deletes all the resources from RAZEE_NAMESPACE
func (r *ReconcileRazeeDeployment) completeUninstall(req *marketplacev1alpha1.RazeeDeployment) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Starting complete uninstall of razee")

	jobName := types.NamespacedName{
		Name:      RAZEE_UNINSTALL_NAME,
		Namespace: req.Namespace,
	}

	// Deploy a job to delete razee if we need to
	// Deploy a job to delete razee if we need to
	if req.Status.RazeeJobInstall != nil {
		foundJob := batch.Job{}
		reqLogger.Info("razee was installed; finding uninstall job")
		err := r.client.Get(context.TODO(), jobName, &foundJob)
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
	return reconcile.Result{}, nil
}

// partialUninstall() deletes the watch-keeper ConfigMap and then the watch-keeper Deployment
func (r *ReconcileRazeeDeployment) partialUninstall(
	req *marketplacev1alpha1.RazeeDeployment,
) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Starting partial uninstall of razee")

	reqLogger.Info("Deleting rr")
	rrUpdate := &unstructured.Unstructured{}
	rrUpdate.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deploy.razee.io",
		Kind:    "RemoteResource",
		Version: "v1alpha2",
	})

	err := r.client.Get(context.Background(), types.NamespacedName{
		Name:      "razeedeploy-auto-update",
		Namespace: *req.Spec.TargetNamespace,
	}, rrUpdate)

	found := true
	if err != nil {
		if !errors.IsNotFound(err) {
			reqLogger.Error(err, "razeedeploy-auto-update not found with error")
			return reconcile.Result{}, err
		}
		if errors.IsNotFound(err) {
			found = false
		}
	}

	deletePolicy := metav1.DeletePropagationForeground

	if found {
		err := r.client.Delete(context.TODO(), rrUpdate, client.PropagationPolicy(deletePolicy))
		if err != nil {
			if !errors.IsNotFound(err) {
				reqLogger.Error(err, "could not delete watch-keeper rr resource")
				return reconcile.Result{}, err
			}
		}
	}

	watchKeeperConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-config",
			Namespace: *req.Spec.TargetNamespace,
		},
	}
	reqLogger.Info("deleting watch-keeper configMap")
	err = r.client.Delete(context.TODO(), watchKeeperConfig)
	if err != nil {
		if !errors.IsNotFound(err) {
			reqLogger.Error(err, "could not delete watch-keeper configmap")
			return reconcile.Result{}, err
		}
	}

	watchKeeperDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper",
			Namespace: *req.Spec.TargetNamespace,
		},
	}
	reqLogger.Info("deleting watch-keeper deployment")
	err = r.client.Delete(context.TODO(), watchKeeperDeployment, client.PropagationPolicy(deletePolicy))
	if err != nil {
		if !errors.IsNotFound(err) {
			reqLogger.Error(err, "could not delete watch-keeper deployment")
			return reconcile.Result{}, err
		}
	}

	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), RAZEE_DEPLOYMENT_FINALIZER))
	err = r.client.Update(context.TODO(), req)
	if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("Partial uninstall of razee is complete")
	return reconcile.Result{}, nil
}
