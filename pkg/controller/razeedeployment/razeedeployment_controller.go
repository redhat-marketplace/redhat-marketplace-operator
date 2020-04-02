package razeedeployment

import (
	"context"
	"fmt"
	"reflect"
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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
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
)

var (
	log                          = logf.Log.WithName("controller_razeedeployment")
	razeeFlagSet                 *pflag.FlagSet
	missingValuesFromSecretSlice      = make([]string, 0, 7)
	razeePrerequisitesCreated         = make([]string, 0, 7)
	localSecretVarsPopulated     bool = false
	redHatMarketplaceSecretFound bool = false
	RELATED_IMAGE_RAZEE_JOB           = "RELATED_IMAGE_RAZEE_JOB"
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

	razeeOpts := &RazeeOpts{
		RazeeDashUrl:  viper.GetString("razeedash-url"),
		RazeeJobImage: viper.GetString("razee-job-image"),
	}

	/******************************************************************************
	CHECK THE INSTANCE FOR VALUES PASSED DOWN FROM MARKETPLACE CONFIG
	check the instance for rhmSecretNameNonNil
	check the instance for *clusterUUID
	/******************************************************************************/
	rhmSecretName := "rhm-operator-secret"
	clusterUUID := &instance.Spec.ClusterUUID

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
			reqLogger.Error(err, "Failed to find operator secret")
			return reconcile.Result{}, nil
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
	searchItems := []string{IBM_COS_READER_KEY_FIELD, BUCKET_NAME_FIELD, IBM_COS_URL_FIELD, RAZEE_DASH_ORG_KEY_FIELD, CHILD_RRS3_YAML_FIELD, RAZEE_DASH_URL_FIELD, FILE_SOURCE_URL_FIELD}
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
		PROCEED WITH CREATING RAZEEDEPLOY-JOB? YES/NO
		do we have all the fields from rhm-secret ? (combined secret)
		check that we can continue with applying the razee job
		if the job has already run exit
		if there are still missing resources exit
	/******************************************************************************/
	if instance.Status.JobState.Succeeded == 1 || len(*instance.Status.MissingValuesFromSecret) > 0 {
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
			reqLogger.Info("Razee namespace created successfully")
			*instance.Status.RazeePrerequisitesCreated = append(*instance.Status.RazeePrerequisitesCreated, fmt.Sprintf("%v namespace", razeeNamespace.Name))
		} else {
			reqLogger.Error(err, "Failed to get razee ns.")
			return reconcile.Result{}, err
		}
	}
	if &razeeNamespace != nil {
		reqLogger.Info("razee namespace already exists - overwriting")
		razeeNamespace.ObjectMeta.Name = "razee"
		err = r.client.Update(context.TODO(), &razeeNamespace)
		if err != nil {
			reqLogger.Error(err, "Failed to update razee namespace")
		}
	}

	watchKeeperNonNamespace := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-non-namespaced", Namespace: "razee"}, &watchKeeperNonNamespace)
	// else create
	if err != nil {
		if errors.IsNotFound(err) {
			watchKeeperNonNamespace = *r.MakeWatchKeeperNonNamespace()
			err = r.client.Create(context.TODO(), &watchKeeperNonNamespace)
			if err != nil {
				reqLogger.Error(err, "Failed to create watch-keeper-non-namespace")
				return reconcile.Result{}, err
			}
			*instance.Status.RazeePrerequisitesCreated = append(*instance.Status.RazeePrerequisitesCreated, watchKeeperNonNamespace.Name)
			reqLogger.Info("watch-keeper-non-namespace created successfully")
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

	// // apply watch-keeper-limit-poll config map
	watchKeeperLimitPoll := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-limit-poll", Namespace: "razee"}, &watchKeeperLimitPoll)
	if err != nil {
		if errors.IsNotFound(err) {
			watchKeeperLimitPoll = *r.MakeWatchKeeperLimitPoll()
			err = r.client.Create(context.TODO(), &watchKeeperLimitPoll)
			if err != nil {
				reqLogger.Error(err, "Failed to create watch-keeper-limit-poll config map")
				return reconcile.Result{}, err
			}
			*instance.Status.RazeePrerequisitesCreated = append(*instance.Status.RazeePrerequisitesCreated, watchKeeperLimitPoll.Name)
			reqLogger.Info("watch-keeper-limit-poll config map created successfully")
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

	// // create razee-cluster-metadata
	razeeClusterMetaData := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "razee-cluster-metadata", Namespace: "razee"}, &razeeClusterMetaData)
	if err != nil {
		if errors.IsNotFound(err) {
			razeeClusterMetaData = *r.MakeRazeeClusterMetaData(*clusterUUID)
			err = r.client.Create(context.TODO(), &razeeClusterMetaData)
			if err != nil {
				reqLogger.Error(err, "Failed to create razee-cluster-metadata config map")
			}
			*instance.Status.RazeePrerequisitesCreated = append(*instance.Status.RazeePrerequisitesCreated, razeeClusterMetaData.Name)
			reqLogger.Info("razee-cluster-metadata config map created successfully")
		} else {
			reqLogger.Error(err, "Failed to get razee-cluster-metadata config map.")
			return reconcile.Result{}, err
		}
	}
	if &razeeClusterMetaData != nil {
		reqLogger.Info("razee-cluster-metadata config map already exists - overwriting")
		razeeClusterMetaData := r.MakeRazeeClusterMetaData(*clusterUUID)
		err = r.client.Update(context.TODO(), razeeClusterMetaData)
		if err != nil {
			reqLogger.Error(err, "Failed to overwrite razee-cluster-metadata config map")
		}
	}

	// return reconcile.Result{}, nil
	// create watch-keeper-config
	watchKeeperConfig := r.MakeWatchKeeperConfig(rhmOperatorSecretValues)
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: watchKeeperConfig.Name, Namespace: "razee"}, watchKeeperConfig)
	// else create
	if err != nil {
		if errors.IsNotFound(err) {
			watchKeeperConfig = r.MakeWatchKeeperConfig(rhmOperatorSecretValues)
			err = r.client.Create(context.TODO(), watchKeeperConfig)
			if err != nil {
				reqLogger.Error(err, "Failed to create watch-keeper-config")
			}
			*instance.Status.RazeePrerequisitesCreated = append(*instance.Status.RazeePrerequisitesCreated, watchKeeperConfig.Name)
			reqLogger.Info("watch-keeper-config created successfully")
		} else {
			reqLogger.Error(err, "Failed to get watch-keeper-config.")
			return reconcile.Result{}, err
		}
	}
	if watchKeeperConfig != nil {
		reqLogger.Info("watch-keeper-config already exists - overwriting")
		watchKeeperConfig = r.MakeWatchKeeperConfig(rhmOperatorSecretValues)
		err = r.client.Update(context.TODO(), watchKeeperConfig)
		if err != nil {
			reqLogger.Error(err, "Failed to update watch-keeper-config")
		}
		reqLogger.Info("watch-keeper-config updated successfully")
	}

	// create watch-keeper-secret
	watchKeeperSecret := corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-secret", Namespace: "razee"}, &watchKeeperSecret)
	// else create
	if err != nil {
		if errors.IsNotFound(err) {
			watchKeeperSecret = *r.MakeWatchKeeperSecret(rhmOperatorSecretValues)
			err = r.client.Create(context.TODO(), &watchKeeperSecret)
			if err != nil {
				reqLogger.Error(err, "Failed to create watch-keeper-secret")
			}
			*instance.Status.RazeePrerequisitesCreated = append(*instance.Status.RazeePrerequisitesCreated, watchKeeperSecret.Name)
			reqLogger.Info("watch-keeper-secret created successfully")
		} else {
			reqLogger.Error(err, "Failed to get watch-keeper-secret.")
			return reconcile.Result{}, err
		}
	}
	if &watchKeeperSecret != nil {
		reqLogger.Info("watch-keeper-secret already exists - overwriting")
		watchKeeperSecret = *r.MakeWatchKeeperSecret(rhmOperatorSecretValues)
		err = r.client.Update(context.TODO(), &watchKeeperSecret)
		if err != nil {
			reqLogger.Error(err, "Failed to update watch-keeper-secret")
		}
		reqLogger.Info("watch-keeper-secret updated successfully")
	}

	// create watch-keeper-config
	ibmCosReaderKey := corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "ibm-cos-reader-key", Namespace: "razee"}, &ibmCosReaderKey)
	if err != nil {
		if errors.IsNotFound(err) {
			ibmCosReaderKey = *r.MakeCOSReaderSecret(rhmOperatorSecretValues)
			err = r.client.Create(context.TODO(), &ibmCosReaderKey)
			if err != nil {
				reqLogger.Error(err, "Failed to create ibm-cos-reader-key")
			}
			*instance.Status.RazeePrerequisitesCreated = append(*instance.Status.RazeePrerequisitesCreated, ibmCosReaderKey.Name)
			reqLogger.Info("ibm-cos-reader-key created successfully")
		} else {
			reqLogger.Error(err, "Failed to get ibm-cos-reader-key.")
			return reconcile.Result{}, err
		}
	}
	if &ibmCosReaderKey != nil {
		ibmCosReaderKey = *r.MakeCOSReaderSecret(rhmOperatorSecretValues)
		reqLogger.Info("ibm-cos-reader-key already exists - overwriting")
		err = r.client.Update(context.TODO(), &ibmCosReaderKey)
		if err != nil {
			reqLogger.Error(err, "Failed to update ibm-cos-reader-key")
		}
		reqLogger.Info("ibm-cos-reader-key updated successfully")
	}

	// if everything gets applied without errors update the status
	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update status")
		return reconcile.Result{}, err
	}

	reqLogger.Info("prerequisite resource have been created")

	/******************************************************************************
	CREATE THE RAZEE JOB
	/******************************************************************************/
	job := r.MakeRazeeJob(request, razeeOpts, rhmOperatorSecretValues)

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
		parentRRS3 := r.MakeParentRemoteResourceS3(rhmOperatorSecretValues)
		err = r.client.Create(context.TODO(), parentRRS3)
		if err != nil {
			reqLogger.Error(err, "Failed to create parentRRS3")
		}
		*instance.Status.RazeePrerequisitesCreated = append(*instance.Status.RazeePrerequisitesCreated, parentRRS3.GetName())
		reqLogger.Info("parentRRS3 created successfully")

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

	reqLogger.Info("End of reconcile")
	return reconcile.Result{}, nil

}

func (r *ReconcileRazeeDeployment) MakeRazeeJob(request reconcile.Request, opts *RazeeOpts, rhmOperatorSecretValues RhmOperatorSecretValues) *batch.Job {
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
						Image:   DEFAULT_RAZEE_JOB_IMAGE,
						Command: []string{"node", "src/install", "--namespace=razee"},
						Args:    []string{fmt.Sprintf("--file-source=%v", rhmOperatorSecretValues.fileSourceUrl), "--autoupdate"},
					}},
					RestartPolicy: "Never",
				},
			},
		},
	}
}

func (r *ReconcileRazeeDeployment) MakeRazeeClusterMetaData(uuid string) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "razee-cluster-metadata",
			Namespace: "razee",
			Labels: map[string]string{
				"razee/cluster-metadata": "true",
				"razee/watch-resource":   "lite",
			},
		},
		Data: map[string]string{"name": uuid},
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

func (r *ReconcileRazeeDeployment) MakeWatchKeeperConfig(rhmOperatorSecretValues RhmOperatorSecretValues) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-config",
			Namespace: "razee",
		},
		Data: map[string]string{"RAZEEDASH_URL": rhmOperatorSecretValues.razeeDashUrl, "START_DELAY_MAX": "0"},
	}
}

func (r *ReconcileRazeeDeployment) MakeWatchKeeperSecret(rhmOperatorSecretValues RhmOperatorSecretValues) *corev1.Secret {
	key := rhmOperatorSecretValues.razeeDashOrgKey
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-secret",
			Namespace: "razee",
		},
		Data: map[string][]byte{"RAZEEDASH_ORG_KEY": []byte(key)},
	}
}

func (r *ReconcileRazeeDeployment) MakeCOSReaderSecret(rhmOperatorValues RhmOperatorSecretValues) *corev1.Secret {
	cosApiKey := rhmOperatorValues.ibmCosReaderKey
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ibm-cos-reader-key",
			Namespace: "razee",
		},
		Data: map[string][]byte{"accesskey": []byte(cosApiKey)},
	}
}

func (r *ReconcileRazeeDeployment) MakeParentRemoteResourceS3(rhmOperatorSecretValues RhmOperatorSecretValues) *unstructured.Unstructured {
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
					map[string]map[string]string{"options": {"url": rhmOperatorSecretValues.ibmCosFullUrl}},
				},
			},
		},
	}
}
