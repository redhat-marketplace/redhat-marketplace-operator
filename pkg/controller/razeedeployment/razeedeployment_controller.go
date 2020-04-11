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
	DEFAULT_RAZEE_JOB_IMAGE    = "quay.io/razee/razeedeploy-delta:0.3.1"
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
	IBM_COS_FULL_URL_FIELD = "ibmCosFullUrl"

)

var (
	log                          = logf.Log.WithName("controller_razeedeployment")
	razeeFlagSet                 *pflag.FlagSet
	RELATED_IMAGE_RAZEE_JOB           = "RELATED_IMAGE_RAZEE_JOB"
	clusterUUID = ""

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

	pred := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
		  return e.Meta.GetName() == RHM_OPERATOR_SECRET_NAME
		},
		CreateFunc: func(e event.CreateEvent) bool{
			return e.Meta.GetName() == RHM_OPERATOR_SECRET_NAME 
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.MetaOld.GetName() == RHM_OPERATOR_SECRET_NAME
		},
	}

	err = c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestForObject{},
		pred,
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
	razeePrerequisitesCreated := make([]string, 0, 7)
	// localSecretVarsPopulated := false

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RazeeDeployment")

	switch request.Name {
		case  RHM_OPERATOR_SECRET_NAME:
			_,err := r.reconcileRhmOperatorSecret(&request)
			if err != nil{
				reqLogger.Error(err,"Failed to reconcile secret")
			}

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
			/******************************************************************************
			CHECK THE INSTANCE FOR VALUES PASSED DOWN FROM MARKETPLACE CONFIG
			check the instance for rhmSecretNameNonNil
			check the instance for *clusterUUID
			/******************************************************************************/
			clusterUUID = instance.Spec.ClusterUUID
				
			// Check if the RazeeDeployment instance is being marked for deletion
			isMarkedForDeletion := instance.GetDeletionTimestamp() != nil
			if isMarkedForDeletion {
				if utils.Contains(instance.GetFinalizers(), razeeDeploymentFinalizer) {
					//Run finalization logic for the razeeDeploymentFinalizer.
					//If it fails, don't remove the finalizer so we can retry during the next reconcile
					result, err := r.finalizeRazeeDeployment(instance, request.Namespace)

					if err != nil {
						return reconcile.Result{}, err
					}

					if result != nil {
						return *result, nil
					}

					// Remove the razeeDeploymentFinalizer
					// Once all finalizers are removed, the object will be deleted
					instance.SetFinalizers(utils.RemoveKey(instance.GetFinalizers(), razeeDeploymentFinalizer))
					err = r.client.Update(context.TODO(), instance)
					if err != nil {
						return reconcile.Result{}, err
					}
				}
				return reconcile.Result{}, nil
			}

			
			// Adding a finalizer to this CR
			if !utils.Contains(instance.GetFinalizers(), razeeDeploymentFinalizer) {
				if err := r.addFinalizer(instance, request.Namespace); err != nil {
					return reconcile.Result{}, err
				}
			}

			/******************************************************************************
			PROCEED WITH CREATING RAZEEDEPLOY-JOB? YES/NO
			do we have all the fields from rhm-secret ? (combined secret)
			check that we can continue with applying the razee job
			if the job has already run exit
			if there are still missing resources exit
			/******************************************************************************/
			
	
			if instance.Spec.DeploySecretValues == nil {
				reqLogger.Info("rhm-operator-secret has not been applied")
				req := reconcile.Request{
						types.NamespacedName{
						Name: *instance.Spec.DeploySecretName,
						Namespace: request.Namespace,
					},
				}
				r.reconcileRhmOperatorSecret(&req)
				return reconcile.Result{RequeueAfter: time.Second * 30}, nil
			}

			if instance.Spec.DeploySecretValues != nil{
				correctList := []string{RAZEE_DASH_ORG_KEY_FIELD,BUCKET_NAME_FIELD,IBM_COS_URL_FIELD,CHILD_RRS3_YAML_FIELD,IBM_COS_READER_KEY_FIELD,RAZEE_DASH_URL_FIELD,FILE_SOURCE_URL_FIELD}
				if missingItems := utils.CheckMapKeys(instance.Spec.DeploySecretValues,correctList);len(missingItems) > 0  {
					reqLogger.Info("Missing required razee configuration values")

					instance.Status.MissingDeploySecretValues = &missingItems
					err = r.client.Status().Update(context.TODO(), instance)		
					if err != nil {
						reqLogger.Error(err, "Failed to update Status.MissingDeploySecretValues")
					}

					req := reconcile.Request{types.NamespacedName{
							Name: *instance.Spec.DeploySecretName,
							Namespace: request.Namespace,
						},
					}
					_,err := r.reconcileRhmOperatorSecret(&req)
					if err != nil{
						reqLogger.Error(err,"Failed to reconcile secret")
					}
					return reconcile.Result{RequeueAfter: time.Second * 30}, nil
				} else {
					// if all the secret values are there, then create the child url and update missing items
					instance.Status.MissingDeploySecretValues = &missingItems
					err = r.client.Status().Update(context.TODO(), instance)		
					if err != nil {
						reqLogger.Error(err, "Failed to update Status.MissingDeploySecretValues")
					}

					url := fmt.Sprintf("%s/%s/%s/%s", instance.Spec.DeploySecretValues[IBM_COS_URL_FIELD], instance.Spec.DeploySecretValues[BUCKET_NAME_FIELD], clusterUUID, instance.Spec.DeploySecretValues[CHILD_RRS3_YAML_FIELD])
					instance.Spec.IbmCosFullUrl = &url
					err = r.client.Update(context.TODO(), instance)		
					if err != nil {
						reqLogger.Error(err, "Failed to update IbmCosFullUrl")
					}		
					reqLogger.Info("All required razee configuration values have been found")
				}
				
			}

			if instance.Status.JobState.Succeeded == 1  {
				reqLogger.Info("RazeeDeployJob has been successfully created")
				return reconcile.Result{}, nil
			}

			/******************************************************************************
			APPLY RAZEE RESOURCES
			/******************************************************************************/
			instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
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
				reqLogger.Info("razee namespace already exists - overwriting")
			}

			razeePrerequisitesCreated = append(razeePrerequisitesCreated, fmt.Sprintf("%v namespace", razeeNamespace.Name))
			if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
				instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
				err = r.client.Status().Update(context.TODO(), instance)
				if err != nil{
					reqLogger.Error(err,"Failed to add razee namespace to Status.razeePrerequisitesCreated:")
				}
			}

			watchKeeperNonNamespace := corev1.ConfigMap{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-non-namespaced", Namespace: "razee"}, &watchKeeperNonNamespace)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info("watch-keeper-non-namespace does not exist - creating")
					watchKeeperNonNamespace = *r.MakeWatchKeeperNonNamespace()
					err = r.client.Create(context.TODO(), &watchKeeperNonNamespace)
					if err != nil {
						reqLogger.Error(err, "Failed to create watch-keeper-non-namespace")
						return reconcile.Result{}, err
					}
					return reconcile.Result{Requeue: true}, nil
				} else {
					reqLogger.Error(err, "Failed to get watch-keeper-non-namespace.")
					return reconcile.Result{}, err
				}
			}
			if &watchKeeperNonNamespace != nil {
				reqLogger.Info("watch-keeper-non-namespace configmap already exists - overwriting")
				watchKeeperNonNamespace = *r.MakeWatchKeeperNonNamespace()
				err = r.client.Update(context.TODO(), &watchKeeperNonNamespace)
				if err != nil {
					reqLogger.Error(err, "Failed to overwrite watch-keeper-non-namespace config map")
				}
			}

			razeePrerequisitesCreated = append(razeePrerequisitesCreated, "watch-keeper-non-namespace" )
			if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
				instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
				err = r.client.Status().Update(context.TODO(), instance)
				if err != nil{
					reqLogger.Error(err,"Failed to add watch-keeper-non-namespace to Status.razeePrerequisitesCreated:")
				}
			}

			// apply watch-keeper-limit-poll config map
			watchKeeperLimitPoll := corev1.ConfigMap{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-limit-poll", Namespace: "razee"}, &watchKeeperLimitPoll)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info("watch-keeper-limit-poll does not exist - creating")
					watchKeeperLimitPoll = *r.MakeWatchKeeperLimitPoll()
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
			if &watchKeeperLimitPoll != nil {
				reqLogger.Info("watch-keeper-limit-poll configmap already exists - overwriting")
				watchKeeperLimitPoll = *r.MakeWatchKeeperLimitPoll()
				err = r.client.Update(context.TODO(), &watchKeeperLimitPoll)
				if err != nil {
					reqLogger.Error(err, "Failed to overwrite watch-keeper-limit-poll config map")
				}
			}

			razeePrerequisitesCreated = append(razeePrerequisitesCreated, "watch-keeper-limit-poll" )
			if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
				instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
				err = r.client.Status().Update(context.TODO(), instance)
				if err != nil{
					reqLogger.Error(err,"Failed to add watch-keeper-limit-poll to Status.razeePrerequisitesCreated:")
				}
			}

			// create razee-cluster-metadata
			razeeClusterMetaData := corev1.ConfigMap{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: "razee-cluster-metadata", Namespace: "razee"}, &razeeClusterMetaData)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info("razee-cluster-metadata does not exist - creating")
					razeeClusterMetaData = *r.MakeRazeeClusterMetaData(instance)
					err = r.client.Create(context.TODO(), &razeeClusterMetaData)
					if err != nil {
						reqLogger.Error(err, "Failed to create razee-cluster-metadata config map")
					}
					reqLogger.Info("razee-cluster-metadata config map created successfully")
					return reconcile.Result{Requeue: true}, nil
				} else {
					reqLogger.Error(err, "Failed to get razee-cluster-metadata config map.")
					return reconcile.Result{}, err
				}
			}
			if &razeeClusterMetaData != nil {
				reqLogger.Info("razee-cluster-metadata config map already exists - overwriting")
				razeeClusterMetaData := r.MakeRazeeClusterMetaData(instance)
				err = r.client.Update(context.TODO(), razeeClusterMetaData)
				if err != nil {
					reqLogger.Error(err, "Failed to overwrite razee-cluster-metadata config map")
				}
			}

			razeePrerequisitesCreated = append(razeePrerequisitesCreated, "razee-cluster-metadata" )
			if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
				instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
				err = r.client.Status().Update(context.TODO(), instance)
				if err != nil{
					reqLogger.Error(err,"Failed to add razee-cluster-metadata to Status.razeePrerequisitesCreated:")
				}
			}

			// create watch-keeper-config
			watchKeeperConfig := corev1.ConfigMap{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-config", Namespace: "razee"}, &watchKeeperConfig)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info("watch-keeper-config does not exist - creating")
					watchKeeperConfig = *r.MakeWatchKeeperConfig(instance)
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
			if &watchKeeperConfig != nil {
				reqLogger.Info("watch-keeper-config already exists - overwriting")
				watchKeeperConfig = *r.MakeWatchKeeperConfig(instance)
				err = r.client.Update(context.TODO(), &watchKeeperConfig)
				if err != nil {
					reqLogger.Error(err, "Failed to update watch-keeper-config")
				}
				reqLogger.Info("watch-keeper-config updated successfully")
			}

			razeePrerequisitesCreated = append(razeePrerequisitesCreated, "watch-keeper-config" )
			if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
				err = r.client.Status().Update(context.TODO(), instance)
				if err != nil{
					reqLogger.Error(err,"Failed to add watch-keeper-config to Status.razeePrerequisitesCreated:")
				}
			}

			// create watch-keeper-secret
			watchKeeperSecret := corev1.Secret{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-secret", Namespace: "razee"}, &watchKeeperSecret)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info("watch-keeper-secret does not exist - creating")
					watchKeeperSecret = *r.MakeWatchKeeperSecret(instance)
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
			if &watchKeeperSecret != nil {
				reqLogger.Info("watch-keeper-secret already exists - overwriting")
				watchKeeperSecret = *r.MakeWatchKeeperSecret(instance)
				err = r.client.Update(context.TODO(), &watchKeeperSecret)
				if err != nil {
					reqLogger.Error(err, "Failed to update watch-keeper-secret")
				}
				reqLogger.Info("watch-keeper-secret updated successfully")
			}

			razeePrerequisitesCreated = append(razeePrerequisitesCreated, "watch-keeper-secret")
			if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
				instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
				err = r.client.Status().Update(context.TODO(), instance)
				if err != nil{
					reqLogger.Error(err,"Failed to add watch-keeper-secret to Status.razeePrerequisitesCreated:")
				}
			}

			// create watch-keeper-config
			ibmCosReaderKey := corev1.Secret{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: "ibm-cos-reader-key", Namespace: "razee"}, &ibmCosReaderKey)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info("ibm-cos-reader-key does not exist - creating")
					ibmCosReaderKey = *r.MakeCOSReaderSecret(instance)
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
			if &ibmCosReaderKey != nil {
				ibmCosReaderKey = *r.MakeCOSReaderSecret(instance)
				reqLogger.Info("ibm-cos-reader-key already exists - overwriting")
				err = r.client.Update(context.TODO(), &ibmCosReaderKey)
				if err != nil {
					reqLogger.Error(err, "Failed to update ibm-cos-reader-key")
				}
				reqLogger.Info("ibm-cos-reader-key updated successfully")
			}

			razeePrerequisitesCreated = append(razeePrerequisitesCreated, "ibm-cos-reader-key")
			if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
				instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
				err = r.client.Status().Update(context.TODO(), instance)
				if err != nil{
					reqLogger.Error(err,"Failed to add ibm-cos-reader-key to Status.razeePrerequisitesCreated:")
				}
			}


			reqLogger.Info("prerequisite resource have been created or updated")

			/******************************************************************************
			CREATE THE RAZEE JOB
			/******************************************************************************/
			job := r.MakeRazeeJob(request, instance)

			// Check if the Job exists already
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "razeedeploy-job",
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

			if len(foundJob.Status.Conditions) == 0 {
				reqLogger.Info("RazeeJob Conditions have not been propagated yet")
				return reconcile.Result{RequeueAfter: time.Second * 30}, nil
			}

			// Update status and conditions
			instance.Status.JobState = foundJob.Status
			for _, jobCondition := range foundJob.Status.Conditions {
				instance.Status.Conditions = &jobCondition
			}

			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update JobState")
				return reconcile.Result{}, nil
			}
			reqLogger.Info("Updated JobState")

			// if the job has a status of succeeded, then apply parent rrs3 delete the job
			if foundJob.Status.Succeeded == 1 {
				// parentRRS3 := r.MakeParentRemoteResourceS3(instance)

				// err = r.client.Create(context.TODO(), parentRRS3)
				// if err != nil {
				// 	reqLogger.Error(err, "Failed to create parentRRS3")
				// }
				// *instance.Status.RazeePrerequisitesCreated = append(*instance.Status.RazeePrerequisitesCreated, parentRRS3.GetName())
				// reqLogger.Info("parentRRS3 created successfully")
				parentRRS3 := &unstructured.Unstructured{}
				parentRRS3.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   "deploy.razee.io",
						Kind:    "RemoteResourceS3",
						Version:  "v1alpha2",
					})
				err = r.client.Get(context.TODO(), client.ObjectKey{Name: "parent", Namespace: "razee",}, parentRRS3)
				if err != nil {
					if errors.IsNotFound(err) {
						reqLogger.Info("parent RRS3 does not exist - creating")
						parentRRS3 = r.MakeParentRemoteResourceS3(instance)
						err = r.client.Create(context.TODO(), parentRRS3)
						if err != nil {
							reqLogger.Error(err, "Failed to create parent RRS3")
						}
						reqLogger.Info("parent RRS3 created successfully")
					} else {
						reqLogger.Error(err, "Failed to get parent RRS3.")
						return reconcile.Result{}, err
					}
				}
				
				if parentRRS3 != nil {
					fmt.Println(parentRRS3.Object)
					parentRRS3.Object["requests"] = []interface{}{
						map[string]map[string]string{"options": {"url": *instance.Spec.IbmCosFullUrl}},
					}
					reqLogger.Info("parent RRS3 already exists - overwriting")
					err = r.client.Update(context.TODO(), parentRRS3)
					if err != nil {
						reqLogger.Error(err, "Failed to update parentRRS3 ")
						return reconcile.Result{}, nil
					}
					reqLogger.Info("parentRRS3 updated successfully")
				}

				razeePrerequisitesCreated = append(razeePrerequisitesCreated, "parentRRS3")
				if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
					err = r.client.Status().Update(context.TODO(), instance)
					if err != nil{
						reqLogger.Error(err,"Failed to add parentRRS3 to Status.razeePrerequisitesCreated")
					}
				}



				err = r.client.Delete(context.TODO(), &foundJob)
				if err != nil {
					reqLogger.Error(err, "Failed to delete job")
					return reconcile.Result{RequeueAfter: time.Second * 30}, nil
				}
				reqLogger.Info("Razeedeploy-job deleted")

				/******************************************************************************
				PATCH RESOURCES FOR DIANEMO
				Patch the Console and Infrastructure resources with the watch-keeper label
				Patch 'razee-cluster-metadata' and add data.name: "max-test-uuid"
				Should only patch if the job has been successfully applied
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
				}
				reqLogger.Info("Found Console resource")
				console.SetLabels(map[string]string{"razee/watch-resource": "lite"})
				err = r.client.Update(context.TODO(), console)
				if err != nil {
					reqLogger.Error(err, "Failed to patch Console resource")
				}
				reqLogger.Info("Patched Console resource")

				// Patch the Infrastructure resource
				reqLogger.Info("finding Infrastructure resource")
				Infrastructure := &unstructured.Unstructured{}
				Infrastructure.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "config.openshift.io",
					Kind:    "Infrastructure",
					Version: "v1",
				})
				err = r.client.Get(context.Background(), client.ObjectKey{
					Name: "cluster",
				}, Infrastructure)

				if err != nil {
					reqLogger.Error(err, "Failed to retrieve Infrastructure resource")
				}
				reqLogger.Info("Found Infrastructure resource")
				Infrastructure.SetLabels(map[string]string{"razee/watch-resource": "lite"})
				err = r.client.Update(context.TODO(), Infrastructure)
				if err != nil {
					reqLogger.Error(err, "Failed to patch Infrastructure resource")
				}
				reqLogger.Info("Patched Infrastructure resource")
				// exit the loop after patches are performed
				return reconcile.Result{}, nil
			}
	}
	
	reqLogger.Info("End of reconcile")
	return reconcile.Result{}, nil
}

func(r *ReconcileRazeeDeployment)reconcileRhmOperatorSecret(request *reconcile.Request)(*reconcile.Result, error){
	MissingDeploySecretValues := make([]string, 0, 8)
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "uestuest.Name", request.Name)
	reqLogger.Info("Beginning of rhm-operator-secret reconcile")

	// retrieve the razee instance
	razeeDeployments := &marketplacev1alpha1.RazeeDeploymentList{}
	err := r.client.List(context.TODO(), razeeDeployments)
	if err != nil {
		reqLogger.Error(err, "Failed to list RazeeDeployments")	
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
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			// report to status that we haven't found the secret
			reqLogger.Info("Updating RedHatMarketplaceSecretFound")
			reqLogger.Error(err, "Failed to find operator secret")
			return nil, nil
		}
		// Error reading the object - requeue the request.
		return nil, err
	}

	/******************************************************************************
	UPDATE SPEC.RAZEE CONFIG VALUES
	/******************************************************************************/
	reqLogger.Info("Adding values to DeploySecretValues")

	// razeeConfigValues,missingItems,err := utils.AddSecretFieldsToStruct(rhmOperatorSecret.Data)
	razeeConfigValues,err := utils.AddSecretFieldsToObj(rhmOperatorSecret.Data)
	if err != nil{
		reqLogger.Error(err,"Failed to get values from deploy secret")
	}

	razeeInstance.Spec.DeploySecretValues = razeeConfigValues
	razeeInstance.Status.MissingDeploySecretValues = &MissingDeploySecretValues
	err = r.client.Update(context.TODO(), &razeeInstance)
	if err != nil {
		reqLogger.Error(err, "Failed to update Spec.DeploySecretValues")
	}

	err = r.client.Status().Update(context.TODO(), &razeeInstance)
	if err != nil {
		reqLogger.Error(err, "Failed to update Spec.DeploySecretValues")
	}
	
	reqLogger.Info("End of rhm-operator-secret reconcile")
	return nil, nil
}

// finalizeRazeeDeployment cleans up resources before the RazeeDeployment CR is deleted
func (r *ReconcileRazeeDeployment) finalizeRazeeDeployment(req *marketplacev1alpha1.RazeeDeployment, namespace string) (*reconcile.Result, error) {
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
			return &reconcile.Result{}, err
		}
	}

	if ! errors.IsNotFound(err) {
		reqLogger.Info("cleaning up install job")
		err := r.client.Delete(context.TODO(), &foundJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "cleaning up install job")
			return &reconcile.Result{}, err
		}
	} else {
		reqLogger.Info("found no job to clean up")
	}

	// Deploy a job to delete razee
	jobName.Name = RAZEE_UNINSTALL_NAME
	foundJob = batch.Job{}
	reqLogger.Info("finding uninstall job")
	err = r.client.Get(context.TODO(), jobName, &foundJob)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating razee-uninstall-job")
		job := r.MakeRazeeUninstallJob(namespace, req)
		err = r.client.Create(context.TODO(), job)
		if err != nil {
			reqLogger.Error(err, "Failed to create Job on cluster")
			return &reconcile.Result{}, err
		}
		reqLogger.Info("job created successfully")
		return &reconcile.Result{RequeueAfter: time.Second * 5}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Job(s) from Cluster")
		return &reconcile.Result{}, err
	}

	reqLogger.Info("found uninstall job")

	if len(foundJob.Status.Conditions) == 0 {
		reqLogger.Info("RazeeUninstallJob Conditions have not been propagated yet")
		return &reconcile.Result{RequeueAfter: time.Second * 30}, nil
	}

	if foundJob.Status.Succeeded < 1 && foundJob.Status.Failed <= 3 {
		reqLogger.Info("RazeeUnisntallJob is not successful")
		return &reconcile.Result{RequeueAfter: time.Second * 30}, nil
	}

	reqLogger.Info("Deleteing uninstall job")
	err = r.client.Delete(context.TODO(), &foundJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if err != nil {
		if !errors.IsNotFound(err) {
			return &reconcile.Result{}, err
		}
	}

	reqLogger.Info("Uninstall job created successfully")
	reqLogger.Info("Successfully finalized RazeeDeployment")
	return nil, nil
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
						Args:    []string{fmt.Sprintf("--file-source=%v", instance.Spec.DeploySecretValues[FILE_SOURCE_URL_FIELD]), "--autoupdate"},
					}},
					RestartPolicy: "Never",
				},
			},
		},
	}
}

// MakeRazeeUninstalllJob returns a Batch.Job which uninstalls razee
func (r *ReconcileRazeeDeployment) MakeRazeeUninstallJob(namespace string, instance *marketplacev1alpha1.RazeeDeployment) *batch.Job {
	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RAZEE_UNINSTALL_NAME,
			Namespace: namespace,
		},
		Spec: batch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "redhat-marketplace-operator",
					Containers: []corev1.Container{{
						Name:    RAZEE_UNINSTALL_NAME,
						Image:   r.opts.RazeeJobImage,
						Command: []string{"node", "src/remove", "--namespace=razee"},
						Args:    []string{fmt.Sprintf("--file-source=%v", instance.Spec.DeploySecretValues[FILE_SOURCE_URL_FIELD]), "--autoupdate"},
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
			Namespace: "razee",
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
			Namespace: "razee",
		},
		Data: map[string]string{"poll": "lite"},
	}
}

//watch-keeper-non-namespace
func (r *ReconcileRazeeDeployment) MakeWatchKeeperLimitPoll() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-limit-poll",
			Namespace: "razee",
		},
		Data: map[string]string{"whitelist": "true", "v1_namespace": "true"},
	}
}

func (r *ReconcileRazeeDeployment) MakeWatchKeeperConfig(instance *marketplacev1alpha1.RazeeDeployment) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-config",
			Namespace: "razee",
		},
		Data: map[string]string{"RAZEEDASH_URL": instance.Spec.DeploySecretValues[RAZEE_DASH_URL_FIELD], "START_DELAY_MAX": "0"},
	}
}

func (r *ReconcileRazeeDeployment) MakeWatchKeeperSecret(instance *marketplacev1alpha1.RazeeDeployment) *corev1.Secret {
	key := instance.Spec.DeploySecretValues[RAZEE_DASH_ORG_KEY_FIELD]
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-secret",
			Namespace: "razee",
		},
		Data: map[string][]byte{"RAZEEDASH_ORG_KEY": []byte(key)},
	}
}

func (r *ReconcileRazeeDeployment) MakeCOSReaderSecret(instance *marketplacev1alpha1.RazeeDeployment) *corev1.Secret {
	cosApiKey := instance.Spec.DeploySecretValues[IBM_COS_READER_KEY_FIELD]
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ibm-cos-reader-key",
			Namespace: "razee",
		},
		Data: map[string][]byte{"accesskey": []byte(cosApiKey)},
	}
}

func (r *ReconcileRazeeDeployment) MakeParentRemoteResourceS3(instance *marketplacev1alpha1.RazeeDeployment) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deploy.razee.io/v1alpha1",
			"kind":       "RemoteResourceS3",
			"metadata": map[string]interface{}{
				"name":      "parent",
				"namespace": "razee",
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
					map[string]map[string]string{"options": {"url": *instance.Spec.IbmCosFullUrl}},
				},
			},
		},
	}
}
