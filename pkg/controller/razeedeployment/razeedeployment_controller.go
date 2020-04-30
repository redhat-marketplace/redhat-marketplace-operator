// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package razeedeployment

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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
	PARENT_RRS3_RESOURCE_NAME = "parent"
	PARENT_RRS3 = "parentRRS3"
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
	RAZEE_WATCH_KEEPER_LABELS = map[string]string{"razee/watch-resource": "lite"}
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
	PROCEED WITH CREATING RAZEE PREREQUISITES? 
	/******************************************************************************/
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

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated,fmt.Sprintf("%v namespace", razeeNamespace.Name)){
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, fmt.Sprintf("%v namespace", razeeNamespace.Name))
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")

		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
	}
	
	watchKeeperNonNamespace := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-non-namespaced", Namespace: *instance.Spec.TargetNamespace}, &watchKeeperNonNamespace)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watch-keeper-non-namespace does not exist - creating")

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

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated,watchKeeperNonNamespace.Name){
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, watchKeeperNonNamespace.Name)
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}
	}
	
	watchKeeperLimitPoll := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-limit-poll", Namespace: *instance.Spec.TargetNamespace}, &watchKeeperLimitPoll)
	if err != nil {
		if errors.IsNotFound(err) {
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

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated,watchKeeperLimitPoll.Name){
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, watchKeeperLimitPoll.Name)
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")

		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}
	
	}

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

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated,razeeClusterMetaData.Name){
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, razeeClusterMetaData.Name)
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}

	}

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

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated,watchKeeperConfig.Name){
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, watchKeeperConfig.Name)
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
	
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
	}
	
	watchKeeperSecret := corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "watch-keeper-secret", Namespace: *instance.Spec.TargetNamespace}, &watchKeeperSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watch-keeper-secret does not exist - creating")
			watchKeeperSecret,err = r.MakeWatchKeeperSecret(instance, request)
			if err != nil{
				return reconcile.Result{},err
			}

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

		updatedWatchKeeperSecret,err := r.MakeWatchKeeperSecret(instance, request)
		if err != nil {
			reqLogger.Error(err, "Failed to build Watch Keeper secret")
			return reconcile.Result{}, err
		}
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

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated,watchKeeperSecret.Name){
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, watchKeeperSecret.Name)
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
	}
	
	ibmCosReaderKey := corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: COS_READER_KEY_NAME, Namespace: *instance.Spec.TargetNamespace}, &ibmCosReaderKey)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("ibm-cos-reader-key does not exist - creating")
			ibmCosReaderKey, err = r.MakeCOSReaderSecret(instance, request)
			if err != nil {
				reqLogger.Error(err, "Failed to build COS Reader Secret")
				return reconcile.Result{}, err
			}

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
		if err != nil {
			reqLogger.Error(err, "Failed to build COS Reader Secret")
			return reconcile.Result{}, err
		}

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

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated,ibmCosReaderKey.Name){
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, ibmCosReaderKey.Name)
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
	
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
		if err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info("Creating razzeedeploy-job")
				err = r.client.Create(context.TODO(), job)
				if err != nil {
					reqLogger.Error(err, "Failed to create Job on cluster")
					return reconcile.Result{}, err
				}
				reqLogger.Info("job created successfully")
				// wait 30 seconds so the job has time to complete
				// not entirely necessary, but the struct on Status.Conditions needs the Conditions in the job to be populated.
				//TODO: requeue or wait for the watch to pick up a change ?
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

			err = r.client.Update(context.TODO(), instance)
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

	// if the job succeeds apply the parentRRS3 and patch the Infrastructure and Console resources
	if instance.Status.JobState.Succeeded == 1 {
		parentRRS3 := &unstructured.Unstructured{}
		parentRRS3.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "deploy.razee.io",
			Kind:    "RemoteResourceS3",
			Version: "v1alpha2",
		})

		err = r.client.Get(context.TODO(), client.ObjectKey{Name: PARENT_RRS3_RESOURCE_NAME, Namespace: *instance.Spec.TargetNamespace}, parentRRS3)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info("parent RRS3 does not exist - creating")

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
				reqLogger.Error(err, "Failed to compare patches")
			}

			if !patchResult.IsEmpty() {
				reqLogger.Info(fmt.Sprintf("Change detected on %v", updatedParentRRS3.GetName()))

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

		if !utils.Contains(instance.Status.RazeePrerequisitesCreated,PARENT_RRS3){
			instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, PARENT_RRS3)
			reqLogger.Info("updating Status.RazeePrerequisitesCreated with parent rrs3")

			err = r.client.Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update status")
				return reconcile.Result{}, err
			}
		}
		
		/******************************************************************************
		PATCH RESOURCES FOR DIANEMO
		Patch the Console and Infrastructure resources with the watch-keeper label
		Patch 'razee-cluster-metadata' with ClusterUUID
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
		consoleLabels := console.GetLabels()

		if !reflect.DeepEqual(consoleLabels, RAZEE_WATCH_KEEPER_LABELS) || consoleLabels == nil {
			console.SetLabels(RAZEE_WATCH_KEEPER_LABELS)
			err = r.client.Update(context.TODO(), console)
			if err != nil {
				reqLogger.Error(err, "Failed to patch Console resource")
				return reconcile.Result{}, err
			}
			reqLogger.Info("Patched Console resource")
		}
		reqLogger.Info("No patch needed on Console resource")

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
		if !reflect.DeepEqual(infrastructureLabels, RAZEE_WATCH_KEEPER_LABELS) || infrastructureLabels == nil {
			infrastructureResource.SetLabels(RAZEE_WATCH_KEEPER_LABELS)
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

func (r *ReconcileRazeeDeployment) reconcileRhmOperatorSecret(instance marketplacev1alpha1.RazeeDeployment, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "request.Name", request.Name)
	reqLogger.Info("Beginning of rhm-operator-secret reconcile")

	if instance.Status.LocalSecretVarsPopulated != nil{
		instance.Status.LocalSecretVarsPopulated = nil
	}

	if instance.Status.RedHatMarketplaceSecretFound != nil{
		instance.Status.RedHatMarketplaceSecretFound = nil
	}
	
	rhmOperatorSecret := corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      *instance.Spec.DeploySecretName,
		Namespace: request.Namespace,
	}, &rhmOperatorSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to find operator secret")
			return reconcile.Result{RequeueAfter: time.Second * 60}, nil
		} else {
			return reconcile.Result{}, err
		}
	}

	razeeConfigurationValues := marketplacev1alpha1.RazeeConfigurationValues{}
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

// Uses the SecretKeySelector struct to to retrieve byte data from a specified key
func (r *ReconcileRazeeDeployment) GetDataFromRhmSecret(request reconcile.Request, sel corev1.SecretKeySelector) ([]byte,error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "request.Name", request.Name)
	reqLogger.Info("Beginning of rhm-operator-secret reconcile")

	rhmOperatorSecret := corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      RHM_OPERATOR_SECRET_NAME,
		Namespace: request.Namespace,
	}, &rhmOperatorSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to find operator secret")
			return nil, err
		}
		return  nil, err
	}
	key, err := utils.ExtractCredKey(&rhmOperatorSecret, sel)
	return key, err
}

func (r *ReconcileRazeeDeployment) MakeWatchKeeperSecret(instance *marketplacev1alpha1.RazeeDeployment, request reconcile.Request) (corev1.Secret,error){
	selector := instance.Spec.DeployConfig.RazeeDashOrgKey
	key, err := r.GetDataFromRhmSecret(request, *selector)

	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-keeper-secret",
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string][]byte{"RAZEEDASH_ORG_KEY": key},
	}, err
}

func (r *ReconcileRazeeDeployment) MakeCOSReaderSecret(instance *marketplacev1alpha1.RazeeDeployment, request reconcile.Request) (corev1.Secret, error) {
	selector := instance.Spec.DeployConfig.IbmCosReaderKey
	key, err := r.GetDataFromRhmSecret(request, *selector)

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
				"name":     PARENT_RRS3_RESOURCE_NAME,
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

// fullUninstall deletes the watch-keeper ConfigMap and then the watch-keeper Deployment
func (r *ReconcileRazeeDeployment) fullUninstall(
	req *marketplacev1alpha1.RazeeDeployment,
) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Starting partial uninstall of razee")

	deletePolicy := metav1.DeletePropagationForeground

	reqLogger.Info("Deleting rrs3")
	rrs3 := &unstructured.Unstructured{}
	rrs3.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deploy.razee.io",
		Kind:    "RemoteResourceS3",
		Version: "v1alpha2",
	})

	reqLogger.Info("Patching rrs3 child")

	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      "child",
		Namespace: *req.Spec.TargetNamespace,
	}, rrs3)

	if err == nil {
		reqLogger.Info("found child rrs3, patching reconcile=false")

		childLabels := rrs3.GetLabels()

		reconcileVal, ok := childLabels["deploy.razee.io/Reconcile"]

		if !ok || (ok && reconcileVal != "false") {
			rrs3.SetLabels(map[string]string{
				"deploy.razee.io/Reconcile": "false",
			})

			err = r.client.Update(context.TODO(), rrs3)
			if err != nil {
				reqLogger.Error(err, "error updating child resource")
			} else {
				// requeue so the label can take affect
				return reconcile.Result{RequeueAfter: 5*time.Second}, nil
			}
		}
	}

	reqLogger.Info("Deleteing rrs3")
	rrs3Names := []string{"parent", "child"}

	for _, rrs3Name := range rrs3Names {
		err = r.client.Get(context.TODO(), types.NamespacedName{
			Name:      rrs3Name,
			Namespace: *req.Spec.TargetNamespace,
		}, rrs3)

		if err == nil {
			err := r.client.Delete(context.TODO(), rrs3, client.PropagationPolicy(deletePolicy))
			if err != nil {
				if !errors.IsNotFound(err) {
					reqLogger.Error(err, "could not delete rrs3 resource", "name", rrs3Name)
				}
			}
		}
	}

	reqLogger.Info("Deleting rr")
	rr := &unstructured.Unstructured{}
	rr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deploy.razee.io",
		Kind:    "RemoteResource",
		Version: "v1alpha2",
	})

	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      "razeedeploy-auto-update",
		Namespace: *req.Spec.TargetNamespace,
	}, rr)

	if err != nil {
		reqLogger.Error(err, "razeedeploy-auto-update not found with error")
	}

	if err == nil {
		err := r.client.Delete(context.TODO(), rr, client.PropagationPolicy(deletePolicy))
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

	serviceAccounts := []string{
		"razeedeploy-sa",
		"watch-keeper-sa",
	}

	for _, saName := range serviceAccounts {
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saName,
				Namespace: *req.Spec.TargetNamespace,
			},
		}
		reqLogger.Info("deleting sa", "name", saName)
		err = r.client.Delete(context.TODO(), serviceAccount, client.PropagationPolicy(deletePolicy))
		if err != nil {
			if err != nil {
				reqLogger.Error(err, "could not delete sa", "name", saName)
			}
		}
	}

	deploymentNames := []string{
		"watch-keeper",
		"clustersubscription",
		"featureflagsetld-controller",
		"managedset-controller",
		"mustachetemplate-controller",
		"remoteresource-controller",
		"remoteresources3-controller",
		"remoteresources3decrypt-controller",
	}

	for _, deploymentName := range deploymentNames {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: *req.Spec.TargetNamespace,
			},
		}
		reqLogger.Info("deleting deployment", "name", deploymentName)
		err = r.client.Delete(context.TODO(), deployment, client.PropagationPolicy(deletePolicy))
		if err != nil {
			if err != nil {
				reqLogger.Error(err, "could not delete deployment", "name", deploymentName)
			}
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
