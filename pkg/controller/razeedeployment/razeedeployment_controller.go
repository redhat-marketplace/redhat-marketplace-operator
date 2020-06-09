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

	"github.com/gotidy/ptr"
	"github.com/operator-framework/operator-sdk/pkg/status"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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

var (
	RAZEE_WATCH_KEEPER_LABELS = map[string]string{"razee/watch-resource": "lite"}
	log                       = logf.Log.WithName("controller_razeedeployment")
	razeeFlagSet              *pflag.FlagSet
	RELATED_IMAGE_RAZEE_JOB   = "RELATED_IMAGE_RAZEE_JOB"
)

func init() {
	razeeFlagSet = pflag.NewFlagSet("razee", pflag.ExitOnError)
	razeeFlagSet.String("razee-job-image", utils.Getenv(RELATED_IMAGE_RAZEE_JOB, utils.DEFAULT_RAZEE_JOB_IMAGE), "image for the razee job")
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

	// This mapFn will queue the default named razeedeployment
	mapFn := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name:      utils.RAZEE_NAME,
					Namespace: a.Meta.GetNamespace(),
				}},
			}
		})

	// Find secret
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			label, _ := utils.GetMapKeyValue(utils.LABEL_RHM_OPERATOR_WATCH)
			// The object doesn't contain label "foo", so the event will be
			// ignored.
			if _, ok := e.MetaOld.GetLabels()[label]; !ok {
				return false
			}

			return e.ObjectOld != e.ObjectNew
		},
		CreateFunc: func(e event.CreateEvent) bool {
			label, _ := utils.GetMapKeyValue(utils.LABEL_RHM_OPERATOR_WATCH)

			if e.Meta.GetName() == utils.RHM_OPERATOR_SECRET_NAME {
				return true
			}

			if _, ok := e.Meta.GetLabels()[label]; !ok {
				return false
			}

			return true
		},
	}

	err = c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: mapFn,
		},
		p)
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

		message := "Razee not enabled"
		instance.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionComplete,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonRazeeInstallFinished,
			Message: message,
		})

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	if instance.Name != utils.RAZEE_NAME {
		reqLogger.Info("Names other than the default are not supported",
			"supportedName", utils.RAZEE_DEPLOY_JOB_NAME,
			"name", instance.Name,
		)

		message := "RazeeDeploy Resource name does not match expected"
		instance.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionComplete,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonRazeeInstallFinished,
			Message: message,
		})

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	if instance.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionInstalling) == nil {
		message := "Razee Install starting"
		instance.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonRazeeStartInstall,
			Message: message,
		})

		_ = r.client.Status().Update(context.TODO(), instance)
		return reconcile.Result{Requeue: true}, nil
	}

	// Adding a finalizer to this CR
	if !utils.Contains(instance.GetFinalizers(), utils.RAZEE_DEPLOYMENT_FINALIZER) {
		if err := r.addFinalizer(instance, request.Namespace); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	}

	// Check if the RazeeDeployment instance is being marked for deletion
	isMarkedForDeletion := instance.GetDeletionTimestamp() != nil
	if isMarkedForDeletion {
		if utils.Contains(instance.GetFinalizers(), utils.RAZEE_DEPLOYMENT_FINALIZER) {
			//Run finalization logic for the RAZEE_DEPLOYMENT_FINALIZER.
			//If it fails, don't remove the finalizer so we can retry during the next reconcile
			return r.partialUninstall(instance)
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
		return reconcile.Result{Requeue: true}, nil
	}

	/******************************************************************************
	PROCEED WITH CREATING RAZEE PREREQUISITES?
	/******************************************************************************/
	// legacy crd value
	if instance.Status.LocalSecretVarsPopulated != nil {
		instance.Status.LocalSecretVarsPopulated = nil
	}

	// legacy crd value
	if instance.Status.RedHatMarketplaceSecretFound != nil {
		instance.Status.RedHatMarketplaceSecretFound = nil
	}

	if instance.Spec.DeployConfig == nil {
		instance.Spec.DeployConfig = &marketplacev1alpha1.RazeeConfigurationValues{}
	}

	secretName := utils.RHM_OPERATOR_SECRET_NAME

	if instance.Spec.DeploySecretName != nil {
		secretName = *instance.Spec.DeploySecretName
	}

	rhmOperatorSecret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      secretName,
		Namespace: request.Namespace,
	}, rhmOperatorSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Failed to find operator secret")
			return reconcile.Result{RequeueAfter: time.Second * 60}, nil
		} else {
			return reconcile.Result{}, err
		}
	}

	if !utils.HasMapKey(rhmOperatorSecret.ObjectMeta.Labels, utils.LABEL_RHM_OPERATOR_WATCH) {
		if rhmOperatorSecret.ObjectMeta.Labels == nil {
			rhmOperatorSecret.ObjectMeta.Labels = make(map[string]string)
		}

		utils.SetMapKeyValue(rhmOperatorSecret.ObjectMeta.Labels, utils.LABEL_RHM_OPERATOR_WATCH)

		err := r.client.Update(context.TODO(), rhmOperatorSecret)
		if err != nil {
			reqLogger.Error(err, "Failed to update Spec.DeploySecretValues")
			return reconcile.Result{}, err
		}
	}

	razeeConfigurationValues := marketplacev1alpha1.RazeeConfigurationValues{}
	razeeConfigurationValues, missingItems, err := utils.AddSecretFieldsToStruct(rhmOperatorSecret.Data, *instance)

	if !utils.Equal(instance.Status.MissingDeploySecretValues, missingItems) ||
		!reflect.DeepEqual(instance.Spec.DeployConfig, &razeeConfigurationValues) {
		instance.Status.MissingDeploySecretValues = missingItems
		instance.Spec.DeployConfig = &razeeConfigurationValues

		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update Spec.DeploySecretValues")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Updated instance deployconfig")
		return reconcile.Result{Requeue: true}, nil
	}

	if len(instance.Status.MissingDeploySecretValues) > 0 {
		reqLogger.Info("Missing required razee configuration values, will wait until the secret is updated")
		return reconcile.Result{}, nil
	}

	reqLogger.V(0).Info("all secret values found")

	//construct the childURL
	url := fmt.Sprintf("%s/%s/%s/%s", instance.Spec.DeployConfig.IbmCosURL, instance.Spec.DeployConfig.BucketName, instance.Spec.ClusterUUID, instance.Spec.DeployConfig.ChildRSS3FIleName)

	if instance.Spec.ChildUrl == nil ||
		(instance.Spec.ChildUrl != nil && *instance.Spec.ChildUrl != url) {
		instance.Spec.ChildUrl = &url
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update ChildUrl")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Updated instance for childUrl")
		return reconcile.Result{Requeue: true}, nil
	}

	// Update the Spec TargetNamespace
	reqLogger.V(0).Info("All required razee configuration values have been found")

	/******************************************************************************
	APPLY OR UPDATE RAZEE RESOURCES
	/******************************************************************************/
	
	// RemoteResourceS3 controller
	rrs3Deployment := &appsv1.Deployment{}
	reqLogger.V(0).Info("Finding RemoteResourceS3 Pod")
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.REMOTE_RESOURCE_S3_NAME,
		Namespace: request.Namespace,
	}, rrs3Deployment)
	if errors.IsNotFound(err) {
		reqLogger.V(0).Info("Creating RemoteResourceS3 deployment")
		rrs3Deployment = r.makeRemoteResourceS3Deployment(instance)
		err = r.client.Create(context.TODO(), rrs3Deployment)
		if err != nil {
			reqLogger.Error(err, "Failed to create RemoteResourceS3 deployment on cluster")
			return reconcile.Result{}, err
		}
		reqLogger.Info("RemoteResourceS3 deployment created successfully")

		message := "RemoteResourceS3 install starting"
		instance.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonRazeeRemoteResourceS3Start,
			Message: message,
		})

		_ = r.client.Status().Update(context.TODO(), instance)

		return reconcile.Result{Requeue: true}, nil

	} else if err != nil {
		reqLogger.Error(err, "Failed to get RemoteResourceS3 from Cluster")
		return reconcile.Result{}, err
	}

	if err := controllerutil.SetControllerReference(instance, rrs3Deployment, r.scheme); err != nil {
		reqLogger.Error(err, "Failed to set controller reference")
		return reconcile.Result{}, err
	}

	razeeNamespace := &corev1.Namespace{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: *instance.Spec.TargetNamespace}, razeeNamespace)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err,
				"targetNamespace does not exist, if you woult like to install into it you will need to create it",
				"targetNamespace", *instance.Spec.TargetNamespace)
			razeeNamespace.ObjectMeta.Name = *instance.Spec.TargetNamespace
			return reconcile.Result{RequeueAfter: time.Second * 60}, nil
		} else {
			reqLogger.Error(err, "Failed to get razee ns.")
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.V(0).Info("razee namespace already exists")
	}

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated, fmt.Sprintf("%v namespace", razeeNamespace.Name)) {
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, fmt.Sprintf("%v namespace", razeeNamespace.Name))

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}

		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		return reconcile.Result{Requeue: true}, nil
	}

	// apply watch-keeper-non-namespaced
	watchKeeperNonNamespace := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: utils.WATCH_KEEPER_NON_NAMESPACED_NAME, Namespace: *instance.Spec.TargetNamespace}, &watchKeeperNonNamespace)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)

			watchKeeperNonNamespace = *r.makeWatchKeeperNonNamespace(instance)
			if err := utils.ApplyAnnotation(&watchKeeperNonNamespace); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			err = r.client.Create(context.TODO(), &watchKeeperNonNamespace)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)
				return reconcile.Result{}, err
			}

			message := "watch-keeper-non-namespaced install finished"
			instance.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonWatchKeeperNonNamespacedInstalled,
				Message: message,
			})

			reqLogger.Info("updating condition", "condition", marketplacev1alpha1.ConditionInstalling)
			_ = r.client.Status().Update(context.TODO(), instance)

			return reconcile.Result{Requeue: true}, nil
		} else {
			reqLogger.Error(err, "Failed to get resource", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.V(0).Info("Resource already exists", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)

		updatedWatchKeeperNonNameSpace := r.makeWatchKeeperNonNamespace(instance)
		patchResult, err := utils.RhmPatchMaker.Calculate(&watchKeeperNonNamespace, updatedWatchKeeperNonNameSpace)
		if err != nil {
			reqLogger.Error(err, "Failed to compare patches")
			return reconcile.Result{}, err
		}

		if !patchResult.IsEmpty() {
			reqLogger.V(0).Info("Change detected on resource", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)
			if err := utils.ApplyAnnotation(updatedWatchKeeperNonNameSpace); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			reqLogger.Info("Updating resource", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)
			err = r.client.Update(context.TODO(), updatedWatchKeeperNonNameSpace)
			if err != nil {
				reqLogger.Error(err, "Failed to update resource", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No change detected on resource", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)

	}

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated, utils.WATCH_KEEPER_NON_NAMESPACED_NAME) {
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, utils.WATCH_KEEPER_NON_NAMESPACED_NAME)

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}

		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")
		return reconcile.Result{Requeue: true}, nil
	}

	// apply watch-keeper-limit-poll config map
	watchKeeperLimitPoll := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: utils.WATCH_KEEPER_LIMITPOLL_NAME, Namespace: *instance.Spec.TargetNamespace}, &watchKeeperLimitPoll)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)

			watchKeeperLimitPoll = *r.makeWatchKeeperLimitPoll(instance)
			if err := utils.ApplyAnnotation(&watchKeeperLimitPoll); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			err = r.client.Create(context.TODO(), &watchKeeperLimitPoll)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)
				return reconcile.Result{}, err
			}

			message := "watch-keeper-limit-poll install finished"
			instance.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonWatchKeeperLimitPollInstalled,
				Message: message,
			})

			reqLogger.Info("Resource created successfully", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)
			return reconcile.Result{Requeue: true}, nil
		} else {
			reqLogger.Error(err, "Failed to get resource", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.Info("Resource already exists", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)
		updatedWatchKeeperLimitPoll := r.makeWatchKeeperLimitPoll(instance)
		patchResult, err := utils.RhmPatchMaker.Calculate(&watchKeeperLimitPoll, updatedWatchKeeperLimitPoll)
		if err != nil {
			reqLogger.Error(err, "Failed to calculate patch diff")
			return reconcile.Result{}, err
		}

		if !patchResult.IsEmpty() {
			reqLogger.Info("Updating resource", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)
			if err := utils.ApplyAnnotation(updatedWatchKeeperLimitPoll); err != nil {
				reqLogger.Error(err, "Failed to set annotation ", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)
				return reconcile.Result{}, err
			}
			err = r.client.Update(context.TODO(), updatedWatchKeeperLimitPoll)
			if err != nil {
				reqLogger.Error(err, "Failed to overwrite resource", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No change detected on resource", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)
	}

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated, utils.WATCH_KEEPER_LIMITPOLL_NAME) {
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, utils.WATCH_KEEPER_LIMITPOLL_NAME)
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// create razee-cluster-metadata
	razeeClusterMetaData := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_CLUSTER_METADATA_NAME, Namespace: *instance.Spec.TargetNamespace}, &razeeClusterMetaData)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.RAZEE_CLUSTER_METADATA_NAME)

			razeeClusterMetaData = *r.makeRazeeClusterMetaData(instance)
			if err := utils.ApplyAnnotation(&razeeClusterMetaData); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			err = r.client.Create(context.TODO(), &razeeClusterMetaData)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource ", utils.RAZEE_CLUSTER_METADATA_NAME)
				return reconcile.Result{}, err
			}

			message := "Razee cluster meta data install finished"
			instance.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonRazeeClusterMetaDataInstalled,
				Message: message,
			})

			_ = r.client.Status().Update(context.TODO(), instance)
			reqLogger.Info("Resource created successfully", "resource: ", utils.RAZEE_CLUSTER_METADATA_NAME)
			return reconcile.Result{Requeue: true}, nil
		} else {
			reqLogger.Error(err, "Failed to get resource")
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.V(0).Info("Resource already exists", "resource: ", utils.RAZEE_CLUSTER_METADATA_NAME)

		updatedRazeeClusterMetaData := *r.makeRazeeClusterMetaData(instance)
		patchResult, err := utils.RhmPatchMaker.Calculate(&razeeClusterMetaData, &updatedRazeeClusterMetaData)
		if err != nil {
			reqLogger.Error(err, "Failed to compare patches")
			return reconcile.Result{}, err
		}

		if !patchResult.IsEmpty() {
			reqLogger.V(0).Info("Change detected on resource", "resource: ", utils.RAZEE_CLUSTER_METADATA_NAME)
			if err := utils.ApplyAnnotation(&updatedRazeeClusterMetaData); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}
			reqLogger.Info("Updating resource", "resource: ", utils.RAZEE_CLUSTER_METADATA_NAME)
			err = r.client.Update(context.TODO(), &updatedRazeeClusterMetaData)
			if err != nil {
				reqLogger.Error(err, "Failed to overwrite resource", "resource: ", utils.RAZEE_CLUSTER_METADATA_NAME)
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.Info("No change detected on resource", "resource: ", utils.RAZEE_CLUSTER_METADATA_NAME)
	}

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated, utils.RAZEE_CLUSTER_METADATA_NAME) {
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, utils.RAZEE_CLUSTER_METADATA_NAME)
		reqLogger.V(0).Info("updating Spec.RazeePrerequisitesCreated")

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// create watch-keeper-config
	watchKeeperConfig := corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: utils.WATCH_KEEPER_CONFIG_NAME, Namespace: *instance.Spec.TargetNamespace}, &watchKeeperConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)

			watchKeeperConfig = *r.makeWatchKeeperConfig(instance)
			if err := utils.ApplyAnnotation(&watchKeeperConfig); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			err = r.client.Create(context.TODO(), &watchKeeperConfig)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
				return reconcile.Result{}, err
			}

			message := "watch-keeper-config install finished"
			instance.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonWatchKeeperConfigInstalled,
				Message: message,
			})

			_ = r.client.Status().Update(context.TODO(), instance)

			reqLogger.Info("Resource created successfully", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
			return reconcile.Result{Requeue: true}, nil
		} else {
			reqLogger.Error(err, "Failed to get resource", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.V(0).Info("Resource already exists", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)

		updatedWatchKeeperConfig := *r.makeWatchKeeperConfig(instance)
		patchResult, err := utils.RhmPatchMaker.Calculate(&watchKeeperConfig, &updatedWatchKeeperConfig)
		if err != nil {
			reqLogger.Error(err, "Failed to compare patches")
		}

		if !patchResult.IsEmpty() {
			reqLogger.Info("Change detected on", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
			if err := utils.ApplyAnnotation(&updatedWatchKeeperConfig); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}
			reqLogger.Info("Updating resource", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
			err = r.client.Update(context.TODO(), &updatedWatchKeeperConfig)
			if err != nil {
				reqLogger.Error(err, "Failed to overwrite ", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
				return reconcile.Result{}, err
			}
			reqLogger.Info("Resource updated successfully", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No changed detected on resource", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
	}

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated, utils.WATCH_KEEPER_CONFIG_NAME) {
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, utils.WATCH_KEEPER_CONFIG_NAME)
		reqLogger.Info("updating Spec.RazeePrerequisitesCreated")

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
	}

	// create watch-keeper-secret
	watchKeeperSecret := corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: utils.WATCH_KEEPER_SECRET_NAME, Namespace: *instance.Spec.TargetNamespace}, &watchKeeperSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
			watchKeeperSecret, err = r.makeWatchKeeperSecret(instance, request)
			if err != nil {
				return reconcile.Result{}, err
			}
			err = r.client.Create(context.TODO(), &watchKeeperSecret)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
				return reconcile.Result{}, err
			}

			message := "watch-keeper-secret install finished"
			instance.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonWatchKeeperSecretInstalled,
				Message: message,
			})

			_ = r.client.Status().Update(context.TODO(), instance)

			reqLogger.Info("Resource created successfully", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
			return reconcile.Result{Requeue: true}, nil
		} else {
			reqLogger.Error(err, "Failed to get resource", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.V(0).Info("Resource already exists", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)

		updatedWatchKeeperSecret, err := r.makeWatchKeeperSecret(instance, request)
		if err != nil {
			reqLogger.Error(err, "Failed to build resource", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
			return reconcile.Result{}, err
		}

		if !reflect.DeepEqual(watchKeeperSecret.Data, updatedWatchKeeperSecret.Data) {
			err = r.client.Update(context.TODO(), &watchKeeperSecret)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
				return reconcile.Result{}, err
			}
			reqLogger.Info("Resource updated successfully", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No change detected on resource", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
	}

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated, utils.WATCH_KEEPER_SECRET_NAME) {
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, utils.WATCH_KEEPER_SECRET_NAME)
		reqLogger.V(0).Info("updating Spec.RazeePrerequisitesCreated")

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
	}

	// create ibm-cos-reader-key
	ibmCosReaderKey := corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: utils.COS_READER_KEY_NAME, Namespace: *instance.Spec.TargetNamespace}, &ibmCosReaderKey)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Resource does not exist", "resource: ", utils.COS_READER_KEY_NAME)
			ibmCosReaderKey, err = r.makeCOSReaderSecret(instance, request)
			if err != nil {
				reqLogger.Error(err, "Failed to build resource", "resource: ", utils.COS_READER_KEY_NAME)
				return reconcile.Result{}, err
			}

			err = r.client.Create(context.TODO(), &ibmCosReaderKey)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource", "resource: ", utils.COS_READER_KEY_NAME)
				return reconcile.Result{}, err
			}

			message := "Cos-reader-key install finished"
			instance.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonCosReaderKeyInstalled,
				Message: message,
			})

			_ = r.client.Status().Update(context.TODO(), instance)

			reqLogger.Info("Resource created successfully", "resource: ", utils.COS_READER_KEY_NAME)
			return reconcile.Result{Requeue: true}, nil
		} else {
			reqLogger.Error(err, "Failed to get resource", "resource: ", utils.COS_READER_KEY_NAME)
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.V(0).Info("Resource already exists", "resource: ", utils.COS_READER_KEY_NAME)

		updatedibmCosReaderKey, err := r.makeCOSReaderSecret(instance, request)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to build %v", utils.COS_READER_KEY_NAME))
			return reconcile.Result{}, err
		}

		if !reflect.DeepEqual(ibmCosReaderKey.Data, updatedibmCosReaderKey.Data) {
			err = r.client.Update(context.TODO(), &watchKeeperSecret)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
				return reconcile.Result{}, err
			}
			reqLogger.Info("Resource updated successfully", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No change detected on resource", "resource: ", utils.COS_READER_KEY_NAME)
	}

	reqLogger.V(0).Info("prerequisite resource have been created or updated")
	if !utils.Contains(instance.Status.RazeePrerequisitesCreated, utils.COS_READER_KEY_NAME) {
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, utils.COS_READER_KEY_NAME)
		reqLogger.V(0).Info("updating Spec.RazeePrerequisitesCreated")

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	/******************************************************************************
	APPLY RRS3 AND WATCHKEEPER DEPLOYMENT
	/******************************************************************************/

	parentRRS3 := &marketplacev1alpha1.RemoteResourceS3{}
	err = r.client.Get(context.TODO(), client.ObjectKey{Name: utils.PARENT_RRS3_RESOURCE_NAME, Namespace: *instance.Spec.TargetNamespace}, parentRRS3)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.PARENT_RRS3)
			parentRRS3 := r.makeParentRemoteResourceS3(instance)

			err = r.client.Create(context.TODO(), parentRRS3)
			if err != nil {
				reqLogger.Info("Failed to create resource", "resource: ", utils.PARENT_RRS3)
				return reconcile.Result{}, err
			}
			message := "ParentRRS3 install finished"
			instance.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonParentRRS3Installed,
				Message: message,
			})

			_ = r.client.Status().Update(context.TODO(), instance)

			reqLogger.Info("Resource created successfully", "resource: ", utils.PARENT_RRS3)
			return reconcile.Result{Requeue: true}, nil
		} else {
			reqLogger.Info("Failed to get resource", "resource: ", utils.PARENT_RRS3)
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.V(0).Info("Resource already exists", "resource: ", utils.PARENT_RRS3)

		newParentValues := r.makeParentRemoteResourceS3(instance)
		updatedParentRRS3 := parentRRS3.DeepCopy()
		updatedParentRRS3.Spec = newParentValues.Spec

		if !reflect.DeepEqual(updatedParentRRS3.Spec, parentRRS3.Spec) {
			reqLogger.Info("Change detected on resource", "resource", updatedParentRRS3.GetName(), "update")

			reqLogger.Info("Updating resource", "resource: ", utils.PARENT_RRS3)
			err = r.client.Update(context.TODO(), updatedParentRRS3)
			if err != nil {
				reqLogger.Info("Failed to update resource", "resource: ", utils.PARENT_RRS3)
				return reconcile.Result{}, err
			}
			reqLogger.Info("Resource updated successfully", "resource: ", utils.PARENT_RRS3)
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No change detected on resource", "resource: ", updatedParentRRS3.GetName())
	}

	if !utils.Contains(instance.Status.RazeePrerequisitesCreated, utils.PARENT_RRS3) {
		instance.Status.RazeePrerequisitesCreated = append(instance.Status.RazeePrerequisitesCreated, utils.PARENT_RRS3)
		reqLogger.Info("updating Status.RazeePrerequisitesCreated with parentRRS3")

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	/******************************************************************************
	PATCH RESOURCES FOR DIANEMO
	Patch the Console and Infrastructure resources with the watch-keeper label
	Patch 'razee-cluster-metadata' with ClusterUUID
	/******************************************************************************/
	reqLogger.V(0).Info("finding Console resource")
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

	reqLogger.V(0).Info("Found Console resource")
	consoleLabels := console.GetLabels()

	if !reflect.DeepEqual(consoleLabels, RAZEE_WATCH_KEEPER_LABELS) || consoleLabels == nil {
		console.SetLabels(RAZEE_WATCH_KEEPER_LABELS)
		err = r.client.Update(context.TODO(), console)
		if err != nil {
			reqLogger.Error(err, "Failed to patch Console resource")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Patched Console resource")
		return reconcile.Result{Requeue: true}, nil
	}
	reqLogger.V(0).Info("No patch needed on Console resource")

	reqLogger.V(0).Info("finding Infrastructure resource")
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

	reqLogger.V(0).Info("Found Infrastructure resource")
	infrastructureLabels := infrastructureResource.GetLabels()
	if !reflect.DeepEqual(infrastructureLabels, RAZEE_WATCH_KEEPER_LABELS) || infrastructureLabels == nil {
		infrastructureResource.SetLabels(RAZEE_WATCH_KEEPER_LABELS)
		err = r.client.Update(context.TODO(), infrastructureResource)
		if err != nil {
			reqLogger.Error(err, "Failed to patch Infrastructure resource")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Patched Infrastructure resource")

		return reconcile.Result{Requeue: true}, nil
	}
	reqLogger.V(0).Info("No patch needed on Infrastructure resource")

	message := "Razee install complete"
	change1 := instance.Status.Conditions.SetCondition(status.Condition{
		Type:    marketplacev1alpha1.ConditionInstalling,
		Status:  corev1.ConditionFalse,
		Reason:  marketplacev1alpha1.ReasonRazeeInstallFinished,
		Message: message,
	})

	message = "Razee install complete"
	change2 := instance.Status.Conditions.SetCondition(status.Condition{
		Type:    marketplacev1alpha1.ConditionComplete,
		Status:  corev1.ConditionTrue,
		Reason:  marketplacev1alpha1.ReasonRazeeInstallFinished,
		Message: message,
	})

	if change1 || change2 {
		reqLogger.Info("Updating final status")
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
	}

	reqLogger.Info("End of reconcile")
	return reconcile.Result{}, nil

}

// finalizeRazeeDeployment cleans up resources before the RazeeDeployment CR is deleted
// func (r *ReconcileRazeeDeployment) finalizeRazeeDeployment(req *marketplacev1alpha1.RazeeDeployment) (reconcile.Result, error) {
// 	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
// 	reqLogger.Info("running finalizer")

// 	jobName := types.NamespacedName{
// 		Name:      "razeedeploy-job",
// 		Namespace: req.Namespace,
// 	}

// 	foundJob := batch.Job{}
// 	reqLogger.Info("finding install job")
// 	err := r.client.Get(context.TODO(), jobName, &foundJob)
// 	if err != nil {
// 		if !errors.IsNotFound(err) {
// 			return reconcile.Result{}, err
// 		}
// 	}

// 	if !errors.IsNotFound(err) {
// 		reqLogger.Info("cleaning up install job")
// 		err := r.client.Delete(context.TODO(), &foundJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
// 		if err != nil && !errors.IsNotFound(err) {
// 			reqLogger.Error(err, "cleaning up install job")
// 			return reconcile.Result{}, err
// 		}
// 	} else {
// 		reqLogger.Info("found no job to clean up")
// 	}

// 	// Deploy a job to delete razee if we need to
// 	if req.Status.RazeeJobInstall != nil {
// 		jobName.Name = utils.RAZEE_UNINSTALL_NAME
// 		foundJob = batch.Job{}
// 		reqLogger.Info("razee was installed; finding uninstall job")
// 		err = r.client.Get(context.TODO(), jobName, &foundJob)
// 		if err != nil && errors.IsNotFound(err) {
// 			reqLogger.Info("Creating razee-uninstall-job")
// 			job := r.makeRazeeUninstallJob(req.Namespace, req.Status.RazeeJobInstall)
// 			err = r.client.Create(context.TODO(), job)
// 			if err != nil {
// 				reqLogger.Error(err, "Failed to create Job on cluster")
// 				return reconcile.Result{}, err
// 			}
// 			reqLogger.Info("job created successfully")
// 			return reconcile.Result{RequeueAfter: time.Second * 5}, nil
// 		} else if err != nil {
// 			reqLogger.Error(err, "Failed to get Job(s) from Cluster")
// 			return reconcile.Result{}, err
// 		}

// 		reqLogger.Info("found uninstall job")

// 		if len(foundJob.Status.Conditions) == 0 {
// 			reqLogger.Info("RazeeUninstallJob Conditions have not been propagated yet")
// 			return reconcile.Result{RequeueAfter: time.Second * 30}, nil
// 		}

// 		if foundJob.Status.Succeeded < 1 && foundJob.Status.Failed <= 3 {
// 			reqLogger.Info("RazeeUnisntallJob is not successful")
// 			return reconcile.Result{RequeueAfter: time.Second * 30}, nil
// 		}

// 		reqLogger.Info("Deleteing uninstall job")
// 		err = r.client.Delete(context.TODO(), &foundJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
// 		if err != nil {
// 			if !errors.IsNotFound(err) {
// 				return reconcile.Result{}, err
// 			}
// 		}
// 	}

// 	reqLogger.Info("Uninstall job created successfully")
// 	reqLogger.Info("Successfully finalized RazeeDeployment")

// 	// Remove the RAZEE_DEPLOYMENT_FINALIZER
// 	// Once all finalizers are removed, the object will be deleted
// 	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), utils.RAZEE_DEPLOYMENT_FINALIZER))
// 	err = r.client.Update(context.TODO(), req)
// 	if err != nil {
// 		return reconcile.Result{}, err
// 	}
// 	return reconcile.Result{}, nil
// }

// MakeRazeeUninstalllJob returns a Batch.Job which uninstalls razee
func (r *ReconcileRazeeDeployment) makeRazeeUninstallJob(namespace string, razeeJob *marketplacev1alpha1.RazeeJobInstallStruct) *batch.Job {
	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RAZEE_UNINSTALL_NAME,
			Namespace: namespace,
		},
		Spec: batch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: utils.RAZEE_SERVICE_ACCOUNT,
					Containers: []corev1.Container{{
						Name:    utils.RAZEE_UNINSTALL_NAME,
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
	reqLogger := log.WithValues("Request.Namespace", namespace, "Request.Name", utils.RAZEE_UNINSTALL_NAME)
	reqLogger.Info("Adding Finalizer for the razeeDeploymentFinzliaer")
	razee.SetFinalizers(append(razee.GetFinalizers(), utils.RAZEE_DEPLOYMENT_FINALIZER))

	err := r.client.Update(context.TODO(), razee)
	if err != nil {
		reqLogger.Error(err, "Failed to update RazeeDeployment with the Finalizer")
		return err
	}
	return nil
}

func(r *ReconcileRazeeDeployment) makeRemoteResourceS3Deployment(instance *marketplacev1alpha1.RazeeDeployment)*appsv1.Deployment{
	rep := ptr.Int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.REMOTE_RESOURCE_S3_NAME,
			Namespace: *instance.Spec.TargetNamespace,
			// TODO: change the annotations here ?
			Annotations: map[string]string{
				"razee.io/git-repo": "https://github.com/razee-io/RemoteResourceS3.git",
				"razee.io/commit-sha" : "9c364a79cb6bcc30a2a6354924e904f3d7433d26",
			},
			Labels: map[string]string{
				"razee/watch-resource": "lite",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: rep,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "remoteresources3-controller",
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: "RollingUpdate",
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "remoteresources3-controller",
						"razee/watch-resource": "lite",
					},
					Name: utils.REMOTE_RESOURCE_S3_NAME,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "razeedeploy-sa",
					Containers: []corev1.Container{
						corev1.Container{
							//TODO: update this image
							Image: "quay.io/razee/remoteresources3:0.6.2",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("40m"),
									corev1.ResourceMemory: resource.MustParse("75Mi"),
								},
							},
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "CRD_WATCH_TIMEOUT_SECONDS",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "razeedeploy-overrides",
											},
											Key: "CRD_WATCH_TIMEOUT_SECONDS",
											Optional: ptr.Bool(true),
										},
									},
								},
								corev1.EnvVar{
									Name: "Group",
									Value: "marketplace.redhat.com",
								},
								corev1.EnvVar{
									Name:"Version",
									Value: "v1alpha2",
								},
							},
							ImagePullPolicy: corev1.PullAlways,
							Name: "remoteresources3-controller",
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"sh/liveness.sh"},
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds: 150,
								TimeoutSeconds: 30,
								FailureThreshold: 1,
							},
							VolumeMounts: []corev1.VolumeMount{
								corev1.VolumeMount{
									MountPath: "/usr/src/app/download-cache",
									Name: "cache-volume",
								},
								corev1.VolumeMount{
									MountPath: "/usr/src/app/config",
									Name: "razeedeploy-config",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: "cache-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									//TODO: is this right ?
									Medium: corev1.StorageMediumDefault,
								},
							},
						},
						corev1.Volume{
							Name: "razeedeploy-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "razeedeploy-config",
									},
									DefaultMode: ptr.Int32(420),
									Optional: ptr.Bool(true),
								},
							},
						},
					},
				},
			},
		},
	}
}

// Creates the razee-cluster-metadata config map and applies the TargetNamespace and the ClusterUUID stored on the Razeedeployment cr
func (r *ReconcileRazeeDeployment) makeRazeeClusterMetaData(instance *marketplacev1alpha1.RazeeDeployment) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RAZEE_CLUSTER_METADATA_NAME,
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
func (r *ReconcileRazeeDeployment) makeWatchKeeperNonNamespace(
	instance *marketplacev1alpha1.RazeeDeployment,
) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_NON_NAMESPACED_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string]string{"v1_namespace": "true"},
	}
}

//watch-keeper-non-namespace
func (r *ReconcileRazeeDeployment) makeWatchKeeperLimitPoll(
	instance *marketplacev1alpha1.RazeeDeployment,
) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_LIMITPOLL_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
	}
}

// Creates watchkeeper config and applies the razee-dash-url stored on the Razeedeployment cr
func (r *ReconcileRazeeDeployment) makeWatchKeeperConfig(instance *marketplacev1alpha1.RazeeDeployment) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_CONFIG_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string]string{"RAZEEDASH_URL": instance.Spec.DeployConfig.RazeeDashUrl, "START_DELAY_MAX": "0"},
	}
}

// Uses the SecretKeySelector struct to to retrieve byte data from a specified key
func (r *ReconcileRazeeDeployment) GetDataFromRhmSecret(request reconcile.Request, sel corev1.SecretKeySelector) ([]byte, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "request.Name", request.Name)
	reqLogger.Info("Beginning of rhm-operator-secret reconcile")

	rhmOperatorSecret := corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.RHM_OPERATOR_SECRET_NAME,
		Namespace: request.Namespace,
	}, &rhmOperatorSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to find operator secret")
			return nil, err
		}
		return nil, err
	}
	key, err := utils.ExtractCredKey(&rhmOperatorSecret, sel)
	return key, err
}

// Creates the watch-keeper-secret and applies the razee-dash-org-key stored on the rhm-operator-secret using the selector stored on the Razeedeployment cr
func (r *ReconcileRazeeDeployment) makeWatchKeeperSecret(instance *marketplacev1alpha1.RazeeDeployment, request reconcile.Request) (corev1.Secret, error) {
	selector := instance.Spec.DeployConfig.RazeeDashOrgKey
	key, err := r.GetDataFromRhmSecret(request, *selector)

	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_SECRET_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string][]byte{"RAZEEDASH_ORG_KEY": key},
	}, err
}

// Creates the rhm-cos-reader-key and applies the ibm-cos-reader-key from rhm-operator-secret using the selector stored on the Razeedeployment cr
func (r *ReconcileRazeeDeployment) makeCOSReaderSecret(instance *marketplacev1alpha1.RazeeDeployment, request reconcile.Request) (corev1.Secret, error) {
	selector := instance.Spec.DeployConfig.IbmCosReaderKey
	key, err := r.GetDataFromRhmSecret(request, *selector)

	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.COS_READER_KEY_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string][]byte{"accesskey": []byte(key)},
	}, err
}

// Creates the "parent" RemoteResourceS3 and applies the name of the cos-reader-key and ChildUrl constructed during reconciliation of the rhm-operator-secret
func (r *ReconcileRazeeDeployment) makeParentRemoteResourceS3(instance *marketplacev1alpha1.RazeeDeployment) *marketplacev1alpha1.RemoteResourceS3 {
	return &marketplacev1alpha1.RemoteResourceS3{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RemoteResourceS3",
			APIVersion: "marketplace.redhat.com",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "parent",
			Namespace: *instance.Spec.TargetNamespace,
		},
		Spec: marketplacev1alpha1.RemoteResourceS3Spec{
			Auth: marketplacev1alpha1.Auth{
				Iam: &marketplacev1alpha1.Iam{
					ResponseType: "cloud_iam",
					GrantType: "urn:ibm:params:oauth:grant-type:apikey",
					URL: "https://iam.cloud.ibm.com/identity/token",
					APIKeyRef: marketplacev1alpha1.APIKeyRef{
						ValueFrom: marketplacev1alpha1.ValueFrom{
							SecretKeyRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "ibm-cos-reader-key",
								},
								Key: "accesskey",
							},
						},
					},
				},
			},
			Requests: []marketplacev1alpha1.Request{
				marketplacev1alpha1.Request{
					Options: marketplacev1alpha1.Options{
						URL: *instance.Spec.ChildUrl,
					},
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
				return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
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

	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), utils.RAZEE_DEPLOYMENT_FINALIZER))
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

	foundJob := batch.Job{}
	reqLogger.Info("finding uninstall job")
	jobName := types.NamespacedName{
		Name:      "razeedeploy-job",
		Namespace: req.Namespace,
	}
	err := r.client.Get(context.TODO(), jobName, &foundJob)
	if err == nil || errors.IsNotFound(err) {
		reqLogger.Info("cleaning up install job")
		err = r.client.Delete(context.TODO(), &foundJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "cleaning up install job failed")
		}
	}

	reqLogger.Info("Deleting rr")
	rrUpdate := &unstructured.Unstructured{}
	rrUpdate.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deploy.razee.io",
		Kind:    "RemoteResource",
		Version: "v1alpha2",
	})

	err = r.client.Get(context.Background(), types.NamespacedName{
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

	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), utils.RAZEE_DEPLOYMENT_FINALIZER))
	err = r.client.Update(context.TODO(), req)
	if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("Partial uninstall of razee is complete")
	return reconcile.Result{}, nil
}