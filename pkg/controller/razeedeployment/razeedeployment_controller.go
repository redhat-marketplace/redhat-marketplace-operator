package razeedeployment

import (
	"context"
	"fmt"
	"reflect"
	"time"

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
	razeeDeploymentFinalizer   = "razeedeploy.finalizer.marketplace.redhat.com"
	cosReaderKey               = "rhm-cos-reader-key"
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
)

var (
	log                     = logf.Log.WithName("controller_razeedeployment")
	razeeFlagSet            *pflag.FlagSet
	RELATED_IMAGE_RAZEE_JOB = "RELATED_IMAGE_RAZEE_JOB"
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

type RhmOperatorSecretValues struct {
	razeeDashOrgKey   string
	bucketName        string
	ibmCosUrl         string
	childRRS3FileName string
	ibmCosReaderKey   string
	razeeDashUrl      string
	fileSourceUrl     string
	ibmCosFullUrl     string
}

// Reconcile reads that state of the cluster for a RazeeDeployment object and makes changes based on the state read
// and what is in the RazeeDeployment.Spec
func (r *ReconcileRazeeDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	missingValuesFromSecretSlice := make([]string, 0, 7)
	// razeePrerequisitesCreated         := make([]string, 0, 7)
	localSecretVarsPopulated := false
	redHatMarketplaceSecretFound := false

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
			return r.partialUninstall(instance)
		}

		return reconcile.Result{}, nil
	}

	// Update the Spec TargetNamespace
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
	CHECK THE INSTANCE FOR VALUES PASSED DOWN FROM MARKETPLACE CONFIG
	check the instance for rhmSecretNameNonNil
	check the instance for *clusterUUID
	/******************************************************************************/
	rhmSecretName := "rhm-operator-secret"
	clusterUUID := &instance.Spec.ClusterUUID
	targetNamespace := *instance.Spec.TargetNamespace

	if instance.Spec.DeploySecretName != nil {
		rhmSecretName = *instance.Spec.DeploySecretName
	}

	/******************************************************************************
	CHECK FOR COMBINED SECRET
	check for the presence of the combined secret
	/******************************************************************************/
	combinedSecret := corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      rhmSecretName,
		Namespace: request.Namespace,
	}, &combinedSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			// report to status that we haven't found the secret
			reqLogger.Info("Updating RedHatMarketplaceSecretFound")
			instance.Status.RedHatMarketplaceSecretFound = &redHatMarketplaceSecretFound
			*instance.Status.RedHatMarketplaceSecretFound = false
			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update Status.RedHatMarketplaceSecretFound")
			}
			reqLogger.Info("Failed to find operator secret")
			return reconcile.Result{RequeueAfter: time.Second * 60}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	instance.Status.RedHatMarketplaceSecretFound = &redHatMarketplaceSecretFound
	*instance.Status.RedHatMarketplaceSecretFound = true
	err = r.client.Status().Update(context.TODO(), instance)

	/******************************************************************************
	CHECK FOR MISSING SECRET VALUES
	if the secret is present on the cluster then check the secret for the correct fields
	check for the presence of the combined secret
	/******************************************************************************/
	searchItems := []string{
		IBM_COS_READER_KEY_FIELD,
		BUCKET_NAME_FIELD,
		IBM_COS_URL_FIELD,
		RAZEE_DASH_ORG_KEY_FIELD,
		CHILD_RRS3_YAML_FIELD,
		RAZEE_DASH_URL_FIELD,
		FILE_SOURCE_URL_FIELD,
	}
	missingItems := []string{}
	for _, searchItem := range searchItems {
		if _, ok := combinedSecret.Data[searchItem]; !ok {
			reqLogger.Info("missing value", searchItem)
			missingItems = append(missingItems, searchItem)
		}
	}

	// update missing resources if necessary
	instance.Status.MissingValuesFromSecret = &missingValuesFromSecretSlice
	if !reflect.DeepEqual(missingItems, *instance.Status.MissingValuesFromSecret) {
		reqLogger.Info("Missing Resources Detected on Secret")
		*instance.Status.MissingValuesFromSecret = missingItems
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update missing resources status")
			return reconcile.Result{}, nil
		}
		reqLogger.Info("Updated MissingValuesFromSecret")
	}

	// if there are missing fields on the secret then exit
	if len(missingItems) > 0 {
		reqLogger.Info("missing required prerequisites for razee install")
		return reconcile.Result{}, nil
	}

	/******************************************************************************
	3.) POPULATE THE SECRET VALUES
	if there are not missing fields on the secret then continue to populate vars
	/******************************************************************************/
	reqLogger.Info("Gathering local vars")
	obj, err := utils.AddSecretFieldsToObj(combinedSecret.Data)
	if err != nil {
		reqLogger.Error(err, "Failed to populate secret data into local vars")
		*instance.Status.LocalSecretVarsPopulated = false
	}

	// if no errors, check the obj to make sure there are no nil values
	for key, value := range obj {
		if key == "" || value == "" {
			reqLogger.Error(err, "Local var not populated")
			instance.Status.LocalSecretVarsPopulated = &localSecretVarsPopulated
			*instance.Status.LocalSecretVarsPopulated = false
			return reconcile.Result{}, nil
		}
	}

	rhmOperatorSecretValues := RhmOperatorSecretValues{razeeDashOrgKey: obj[RAZEE_DASH_ORG_KEY_FIELD], bucketName: obj[BUCKET_NAME_FIELD], ibmCosUrl: obj[IBM_COS_URL_FIELD], childRRS3FileName: obj[CHILD_RRS3_YAML_FIELD], ibmCosReaderKey: obj[IBM_COS_READER_KEY_FIELD], razeeDashUrl: obj[RAZEE_DASH_URL_FIELD], fileSourceUrl: obj[FILE_SOURCE_URL_FIELD]}

	// if all fields are present continue to run and update status
	instance.Status.LocalSecretVarsPopulated = &localSecretVarsPopulated
	*instance.Status.LocalSecretVarsPopulated = true
	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update Status.LocalVarsPopulated")
	}
	reqLogger.Info("Local vars have been populated")

	rhmOperatorSecretValues.ibmCosFullUrl = fmt.Sprintf("%s/%s/%s/%s", obj[IBM_COS_URL_FIELD], obj[BUCKET_NAME_FIELD], *clusterUUID, obj[CHILD_RRS3_YAML_FIELD])

	/******************************************************************************
	APPLY RAZEE RESOURCES
	/******************************************************************************/
	razeePrerequisitesCreated := make([]string, 0, 7)
	instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
	razeeNamespace := &corev1.Namespace{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: targetNamespace}, razeeNamespace)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err,
				"targetNamespace does not exist, if you woult like to install into it you will need to create it",
				"targetNamespace", targetNamespace)
			razeeNamespace.ObjectMeta.Name = targetNamespace
			return reconcile.Result{RequeueAfter: time.Second * 60}, nil
		} else {
			reqLogger.Error(err, "Failed to get razee ns.")
			return reconcile.Result{}, err
		}
	}

	razeePrerequisitesCreated = append(razeePrerequisitesCreated, fmt.Sprintf("%v namespace", razeeNamespace.Name))
	if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
		instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
		r.client.Status().Update(context.TODO(), instance)
	}

	watchKeeperNonNamespace := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-non-namespaced", Namespace: targetNamespace}, watchKeeperNonNamespace)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watch-keeper-non-namespace does not exist - creating")
			watchKeeperNonNamespace = r.MakeWatchKeeperNonNamespace(targetNamespace)
			err = r.client.Create(context.TODO(), watchKeeperNonNamespace)
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
	if watchKeeperNonNamespace != nil {
		reqLogger.Info("watch-keeper-non-namespace configmap already exists - overwriting")
		watchKeeperNonNamespace = r.MakeWatchKeeperNonNamespace(targetNamespace)
		err = r.client.Update(context.TODO(), watchKeeperNonNamespace)
		if err != nil {
			reqLogger.Error(err, "Failed to overwrite watch-keeper-non-namespace config map")
		}
	}

	razeePrerequisitesCreated = append(razeePrerequisitesCreated, "watch-keeper-non-namespace")
	if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
		instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
		r.client.Status().Update(context.TODO(), instance)
	}

	// apply watch-keeper-limit-poll config map
	watchKeeperLimitPoll := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-limit-poll", Namespace: targetNamespace}, watchKeeperLimitPoll)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watch-keeper-limit-poll does not exist - creating")
			watchKeeperLimitPoll = r.MakeWatchKeeperLimitPoll(targetNamespace)
			err = r.client.Create(context.TODO(), watchKeeperLimitPoll)
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
	if watchKeeperLimitPoll != nil {
		reqLogger.Info("watch-keeper-limit-poll configmap already exists - overwriting")
		watchKeeperLimitPoll = r.MakeWatchKeeperLimitPoll(targetNamespace)
		err = r.client.Update(context.TODO(), watchKeeperLimitPoll)
		if err != nil {
			reqLogger.Error(err, "Failed to overwrite watch-keeper-limit-poll config map")
		}
	}

	razeePrerequisitesCreated = append(razeePrerequisitesCreated, "watch-keeper-limit-poll")
	if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
		instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
		r.client.Status().Update(context.TODO(), instance)
	}

	// create razee-cluster-metadata
	razeeClusterMetaData := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "razee-cluster-metadata", Namespace: targetNamespace}, razeeClusterMetaData)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("razee-cluster-metadata does not exist - creating")
			razeeClusterMetaData = r.MakeRazeeClusterMetaData(targetNamespace, *clusterUUID)
			err = r.client.Create(context.TODO(), razeeClusterMetaData)
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
		razeeClusterMetaData := r.MakeRazeeClusterMetaData(targetNamespace, *clusterUUID)
		err = r.client.Update(context.TODO(), razeeClusterMetaData)
		if err != nil {
			reqLogger.Error(err, "Failed to overwrite razee-cluster-metadata config map")
		}
	}

	razeePrerequisitesCreated = append(razeePrerequisitesCreated, "razee-cluster-metadata")
	if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
		instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
		r.client.Status().Update(context.TODO(), instance)
	}

	// create watch-keeper-config
	watchKeeperConfig := r.MakeWatchKeeperConfig(targetNamespace, rhmOperatorSecretValues)
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: watchKeeperConfig.Name, Namespace: targetNamespace}, watchKeeperConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watch-keeper-config does not exist - creating")
			watchKeeperConfig = r.MakeWatchKeeperConfig(targetNamespace, rhmOperatorSecretValues)
			err = r.client.Create(context.TODO(), watchKeeperConfig)
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
	if watchKeeperConfig != nil {
		reqLogger.Info("watch-keeper-config already exists - overwriting")
		watchKeeperConfig = r.MakeWatchKeeperConfig(targetNamespace, rhmOperatorSecretValues)
		err = r.client.Update(context.TODO(), watchKeeperConfig)
		if err != nil {
			reqLogger.Error(err, "Failed to update watch-keeper-config")
		}
		reqLogger.Info("watch-keeper-config updated successfully")
	}

	razeePrerequisitesCreated = append(razeePrerequisitesCreated, "watch-keeper-config")
	if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
		instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
		r.client.Status().Update(context.TODO(), instance)
	}

	// create watch-keeper-secret
	watchKeeperSecret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-secret", Namespace: targetNamespace}, watchKeeperSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watch-keeper-secret does not exist - creating")
			watchKeeperSecret = r.MakeWatchKeeperSecret(targetNamespace, rhmOperatorSecretValues)
			err = r.client.Create(context.TODO(), watchKeeperSecret)
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
		watchKeeperSecret = r.MakeWatchKeeperSecret(targetNamespace, rhmOperatorSecretValues)
		err = r.client.Update(context.TODO(), watchKeeperSecret)
		if err != nil {
			reqLogger.Error(err, "Failed to update watch-keeper-secret")
		}
		reqLogger.Info("watch-keeper-secret updated successfully")
	}

	razeePrerequisitesCreated = append(razeePrerequisitesCreated, "watch-keeper-secret")
	if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
		instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
		r.client.Status().Update(context.TODO(), instance)
	}

	// create watch-keeper-config
	ibmCosReaderKey := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: cosReaderKey, Namespace: targetNamespace}, ibmCosReaderKey)
	if err != nil {
		reqLogger.Info("ibm-cos-reader-key does not exist - creating")
		if errors.IsNotFound(err) {
			ibmCosReaderKey = r.MakeCOSReaderSecret(targetNamespace, rhmOperatorSecretValues)
			err = r.client.Create(context.TODO(), ibmCosReaderKey)
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
		ibmCosReaderKey = r.MakeCOSReaderSecret(targetNamespace, rhmOperatorSecretValues)
		reqLogger.Info("ibm-cos-reader-key already exists - overwriting")
		err = r.client.Update(context.TODO(), ibmCosReaderKey)
		if err != nil {
			reqLogger.Error(err, "Failed to update ibm-cos-reader-key")
		}
		reqLogger.Info("ibm-cos-reader-key updated successfully")
	}

	razeePrerequisitesCreated = append(razeePrerequisitesCreated, cosReaderKey)
	if &razeePrerequisitesCreated != instance.Status.RazeePrerequisitesCreated {
		instance.Status.RazeePrerequisitesCreated = &razeePrerequisitesCreated
		r.client.Status().Update(context.TODO(), instance)
	}

	reqLogger.Info("prerequisite resource have been created or updated")

	/******************************************************************************
		PROCEED WITH CREATING RAZEEDEPLOY-JOB? YES/NO
		do we have all the fields from rhm-secret ? (combined secret)
		check that we can continue with applying the razee job
		if the job has already run exit
		if there are still missing resources exit
	/******************************************************************************/
	reqLogger.Info("RazeeDeployJob has been successfully created")

	/******************************************************************************
	CREATE THE RAZEE JOB
	/******************************************************************************/
	if instance.Status.JobState.Succeeded != 1 {
		job := r.MakeRazeeJob(targetNamespace, request, rhmOperatorSecretValues)

		// Check if the Job exists already
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "razeedeploy-job",
				Namespace: request.Namespace,
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

		if len(foundJob.Status.Conditions) == 0 {
			reqLogger.Info("RazeeJob Conditions have not been propagated yet")
			return reconcile.Result{RequeueAfter: time.Second * 30}, nil
		}

		// Update status and conditions
		instance.Status.JobState = foundJob.Status
		for _, jobCondition := range foundJob.Status.Conditions {
			instance.Status.Conditions = &jobCondition
		}
		instance.Status.RazeeJobInstall = &marketplacev1alpha1.RazeeJobInstallStruct{
			RazeeNamespace:  targetNamespace,
			RazeeInstallURL: rhmOperatorSecretValues.fileSourceUrl,
		}

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update jobstate")
			return reconcile.Result{}, err
		}

		reqLogger.Info("Updated JobState")

		err = r.client.Delete(context.TODO(), foundJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil {
			reqLogger.Error(err, "Failed to delete job")
			return reconcile.Result{RequeueAfter: time.Second * 30}, nil
		}
		reqLogger.Info("Razeedeploy-job deleted")
	}

	// if the job has a status of succeeded, then apply parent rrs3 delete the job
	if instance.Status.JobState.Succeeded == 1 {
		parentRRS3 := r.MakeParentRemoteResourceS3(targetNamespace, rhmOperatorSecretValues)

		err = r.client.Create(context.TODO(), parentRRS3)
		if err != nil {
			reqLogger.Error(err, "Failed to create parentRRS3")
		}
		*instance.Status.RazeePrerequisitesCreated = append(*instance.Status.RazeePrerequisitesCreated, parentRRS3.GetName())
		reqLogger.Info("parentRRS3 created successfully")

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

		if err == nil {
			reqLogger.Info("Found Console resource")
			console.SetLabels(map[string]string{"razee/watch-resource": "lite"})
			err = r.client.Update(context.TODO(), console)
			if err != nil {
				reqLogger.Error(err, "Failed to patch Console resource")
			}
			reqLogger.Info("Patched Console resource")
		}

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

		if err == nil{
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

// MakeRazeeJob returns a Batch.Job which installs razee
func (r *ReconcileRazeeDeployment) MakeRazeeJob(
	namespace string,
	request reconcile.Request,
	rhmOperatorSecretValues RhmOperatorSecretValues,
) *batch.Job {
	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "razeedeploy-job",
			Namespace: request.Namespace,
		},
		Spec: batch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: utils.RAZEE_SERVICE_ACCOUNT,
					Containers: []corev1.Container{{
						Name:    "razeedeploy-job",
						Image:   r.opts.RazeeJobImage,
						Command: []string{"node", "src/install", fmt.Sprintf("--namespace=%s", namespace)},
						Args:    []string{fmt.Sprintf("--file-source=%v", rhmOperatorSecretValues.fileSourceUrl), "--autoupdate"},
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

func (r *ReconcileRazeeDeployment) MakeRazeeClusterMetaData(namespace, uuid string) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "razee-cluster-metadata",
			Namespace: namespace,
			Labels: map[string]string{
				"razee/cluster-metadata": "true",
				"razee/watch-resource":   "lite",
			},
		},
		Data: map[string]string{"name": uuid},
	}
}

//watch-keeper-non-namespace
func (r *ReconcileRazeeDeployment) MakeWatchKeeperNonNamespace(
	namespace string,
) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-non-namespaced",
			Namespace: namespace,
		},
		Data: map[string]string{"v1_namespace": "true"},
	}
}

//watch-keeper-non-namespace
func (r *ReconcileRazeeDeployment) MakeWatchKeeperLimitPoll(
	namespace string,
) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-limit-poll",
			Namespace: namespace,
		},
	}
}

func (r *ReconcileRazeeDeployment) MakeWatchKeeperConfig(
	namespace string,
	rhmOperatorSecretValues RhmOperatorSecretValues,
) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-config",
			Namespace: namespace,
		},
		Data: map[string]string{"RAZEEDASH_URL": rhmOperatorSecretValues.razeeDashUrl, "START_DELAY_MAX": "0"},
	}
}

func (r *ReconcileRazeeDeployment) MakeWatchKeeperSecret(
	namespace string,
	rhmOperatorSecretValues RhmOperatorSecretValues,
) *corev1.Secret {
	key := rhmOperatorSecretValues.razeeDashOrgKey
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{"RAZEEDASH_ORG_KEY": []byte(key)},
	}
}

func (r *ReconcileRazeeDeployment) MakeCOSReaderSecret(
	namespace string,
	rhmOperatorValues RhmOperatorSecretValues,
) *corev1.Secret {
	cosApiKey := rhmOperatorValues.ibmCosReaderKey
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cosReaderKey,
			Namespace: namespace,
		},
		Data: map[string][]byte{"accesskey": []byte(cosApiKey)},
	}
}

func (r *ReconcileRazeeDeployment) MakeParentRemoteResourceS3(
	namespace string,
	rhmOperatorSecretValues RhmOperatorSecretValues,
) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deploy.razee.io/v1alpha2",
			"kind":       "RemoteResourceS3",
			"metadata": map[string]interface{}{
				"name":      "parent",
				"namespace": namespace,
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
									"name": cosReaderKey,
									"key":  "accesskey",
								},
							},
						},
					},
				},
				"requests": []interface{}{
					map[string]map[string]string{"options": {"url": rhmOperatorSecretValues.ibmCosFullUrl}},
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
		found = false
		reqLogger.Error(err, "razeedeploy-auto-update not found with error")
	}

	deletePolicy := metav1.DeletePropagationForeground

	if found {
		err := r.client.Delete(context.TODO(), rrUpdate, client.PropagationPolicy(deletePolicy))
		if err != nil {
			if !errors.IsNotFound(err) {
				reqLogger.Error(err, "could not delete watch-keeper rr resource")
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
		if err != nil {
			reqLogger.Error(err, "could not delete watch-keeper configmap")
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
		if err != nil {
			reqLogger.Error(err, "could not delete watch-keeper deployment")
		}
	}

	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), razeeDeploymentFinalizer))
	err = r.client.Update(context.TODO(), req)
	if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("Partial uninstall of razee is complete")
	return reconcile.Result{}, nil
}
