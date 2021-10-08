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

package marketplace

import (
	"context"
	"fmt"
	"reflect"
	"time"

	golangerrors "errors"

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	"golang.org/x/time/rate"
	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	razeeWatchTag            string = "razee/watch-resource"
	razeeWatchTagValueLite   string = "lite"
	razeeWatchTagValueDetail string = "detail"
)

// blank assignment to verify that ReconcileRazeeDeployment implements reconcile.Reconciler
var _ reconcile.Reconciler = &RazeeDeploymentReconciler{}

// RazeeDeploymentReconciler reconciles a RazeeDeployment object
type RazeeDeploymentReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	CC     ClientCommandRunner

	patcher patch.Patcher
	cfg     *config.OperatorConfig
	factory *manifests.Factory
}

func (r *RazeeDeploymentReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *RazeeDeploymentReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.CC = ccp
	return nil
}

func (r *RazeeDeploymentReconciler) InjectPatch(p patch.Patcher) error {
	r.patcher = p
	return nil
}

func (r *RazeeDeploymentReconciler) InjectFactory(f *manifests.Factory) error {
	r.factory = f
	return nil
}

func (r *RazeeDeploymentReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *RazeeDeploymentReconciler) SetupWithManager(mgr manager.Manager) error {

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
		// Ensures RazeeDeployment is only reconciled for appropriate Secrets
		// And not any secrets, regardless of namespace

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
		DeleteFunc: func(e event.DeleteEvent) bool {
			label, _ := utils.GetMapKeyValue(utils.LABEL_RHM_OPERATOR_WATCH)

			if _, ok := e.Meta.GetLabels()[label]; !ok {
				return false
			}

			return true
		},
	}

	pp := predicate.Funcs{
		// Ensures RazeeDeployment reconciles podlist correctly on deletes
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Meta.GetLabels()["owned-by"] == "marketplace.redhat.com-razee"
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Meta.GetLabels()["owned-by"] == "marketplace.redhat.com-razee"
		},
	}

	// Create a new controller
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(predicates.NamespacePredicate(r.cfg.DeployedNamespace)).
		For(&marketplacev1alpha1.RazeeDeployment{}).
		WithOptions(controller.Options{
			Reconciler: r,
			RateLimiter: workqueue.NewMaxOfRateLimiter(
				workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1000*time.Second),
				&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 300)},
			),
		}).
		Watches(&source.Kind{Type: &marketplacev1alpha1.RazeeDeployment{}},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &batch.Job{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &marketplacev1alpha1.RazeeDeployment{},
		}).
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &marketplacev1alpha1.RazeeDeployment{},
		}).
		Watches(&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: mapFn,
			},
			builder.WithPredicates(p)).
		Watches(&source.Kind{Type: &corev1.Pod{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: mapFn,
			},
			builder.WithPredicates(pp)).
		Watches(
			&source.Kind{Type: &marketplacev1alpha1.RemoteResourceS3{}},
			&handler.EnqueueRequestForOwner{
				OwnerType: &marketplacev1alpha1.RazeeDeployment{},
			},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// +kubebuilder:rbac:groups="",resources=configmaps;pods;secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get
// +kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,namespace=system,resources=deployments,verbs=create;update;patch;delete
// +kubebuilder:rbac:groups=batch;extensions,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups="config.openshift.io",resources=consoles;infrastructures;clusterversions,verbs=get;update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=razeedeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=razeedeployments;razeedeployments/finalizers;razeedeployments/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=remoteresources3s,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=remoteresources3s,verbs=get;list;watch;create;update;patch;delete

// Legacy Uninstall

// +kubebuilder:rbac:groups="",resources=serviceaccounts,resourceNames=razeedeploy-sa;watch-keeper-sa,verbs=delete
// +kubebuilder:rbac:groups=apps,resources=deployments,resourceNames=watch-keeper;clustersubscription;featureflagsetld-controller;managedset-controller;mustachetemplate-controller;remoteresource-controller;remoteresources3-controller;remoteresources3decrypt-controller,verbs=delete
// +kubebuilder:rbac:groups=batch;extensions,resources=jobs,resourceNames=razeedeploy-job,verbs=delete
// +kubebuilder:rbac:groups="deploy.razee.io",resources=*,verbs=get;list;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,resourceNames=razeedeploy-admin-cr;redhat-marketplace-razeedeploy,verbs=delete

// Reconcile reads that state of the cluster for a RazeeDeployment object and makes changes based on the state read
// and what is in the RazeeDeployment.Spec
func (r *RazeeDeploymentReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RazeeDeployment")

	// Fetch the RazeeDeployment instance
	instance := &marketplacev1alpha1.RazeeDeployment{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
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

		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status for razee disabled")
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

		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status for invalid razee name")
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

		_ = r.Client.Status().Update(context.TODO(), instance)
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
			return r.fullUninstall(instance)
		}
		return reconcile.Result{}, nil
	}

	if instance.Spec.TargetNamespace == nil {
		if instance.Status.RazeeJobInstall != nil {
			instance.Spec.TargetNamespace = &instance.Status.RazeeJobInstall.RazeeNamespace
		} else {
			instance.Spec.TargetNamespace = &instance.Namespace
		}
		err := r.Client.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}

		reqLogger.Info("set target namespace to", "namespace", instance.Spec.TargetNamespace)
		return reconcile.Result{Requeue: true}, nil
	}

	if instance.Status.LocalSecretVarsPopulated != nil {
		instance.Status.LocalSecretVarsPopulated = nil
	}

	if instance.Status.RedHatMarketplaceSecretFound != nil {
		instance.Status.RedHatMarketplaceSecretFound = nil
	}

	if instance.Status.JobConditions != nil {
		instance.Status.JobConditions = nil
	}

	if instance.Status.JobState != nil {
		instance.Status.JobState = nil
	}

	if instance.Spec.DeployConfig == nil {
		instance.Spec.DeployConfig = &marketplacev1alpha1.RazeeConfigurationValues{}
	}

	if instance.Spec.DeployConfig.FileSourceURL != nil {
		instance.Spec.DeployConfig.FileSourceURL = nil
	}

	secretName := utils.RHM_OPERATOR_SECRET_NAME

	if instance.Spec.DeploySecretName != nil {
		secretName = *instance.Spec.DeploySecretName
	}

	rhmOperatorSecret := &corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
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

		err := r.Client.Update(context.TODO(), rhmOperatorSecret)
		if err != nil {
			reqLogger.Error(err, "Failed to update Spec.DeploySecretValues")
			return reconcile.Result{}, err
		}
	}

	razeeConfigurationValues := marketplacev1alpha1.RazeeConfigurationValues{}
	razeeConfigurationValues, missingItems, err := utils.AddSecretFieldsToStruct(rhmOperatorSecret.Data, *instance)
	if !utils.StringSliceEqual(instance.Status.MissingDeploySecretValues, missingItems) ||
		!reflect.DeepEqual(instance.Spec.DeployConfig, &razeeConfigurationValues) {
		instance.Status.MissingDeploySecretValues = missingItems
		instance.Spec.DeployConfig = &razeeConfigurationValues

		err = r.Client.Update(context.TODO(), instance)
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
		err = r.Client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update ChildUrl")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Updated instance for childUrl")
		return reconcile.Result{Requeue: true}, nil
	}

	// Update the Spec TargetNamespace
	reqLogger.V(0).Info("All required razee configuration values have been found")

	// Check if the RazeeDeployment is disabled, in this case remove the razee deployment and parents3
	rrs3DeploymentEnabled := instance.Spec.Features == nil || instance.Spec.Features.Deployment == nil || *instance.Spec.Features.Deployment
	if !rrs3DeploymentEnabled {
		//razee deployment disabled - if the deployment was found, delete it

		err := r.removeRazeeDeployments(instance)
		if err != nil {
			return reconcile.Result{}, err
		}

		//Deployment is disabled - update status
		reqLogger.V(0).Info("RemoteResourceS3 deployment is disabled")
		//update status to reflect disabled
		message := "RemoteResourceS3 deployment disabled"
		changed := instance.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionDeploymentEnabled,
			Status:  corev1.ConditionFalse,
			Reason:  marketplacev1alpha1.ReasonRhmRemoteResourceS3DeploymentEnabled,
			Message: message,
		})

		if changed {
			reqLogger.Info("RemoteResourceS3 disabled status updated")

			_ = r.Client.Status().Update(context.TODO(), instance)
			r.Client.Get(context.TODO(), request.NamespacedName, instance)
		}
	}

	registrationEnabled := true

	if instance.Spec.Features != nil &&
		instance.Spec.Features.Registration != nil &&
		!*instance.Spec.Features.Registration {
		reqLogger.Info("registration is disabled")
		registrationEnabled = false
	}

	if !registrationEnabled {
		//registration disabled - if watchkeeper is found, delete its deployment
		res, err := r.removeWatchkeeperDeployment(instance)
		reqLogger.Info("watchkeeper delete complete", "res", res, "err", err)
		if res != nil {
			return *res, err
		}

		//Deployment is disabled - update status
		reqLogger.V(0).Info("Registration watchkeeper deployment is disabled")
		//update status to reflect disabled
		message := "Registration deployment disabled"
		changed := instance.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionRegistrationEnabled,
			Status:  corev1.ConditionFalse,
			Reason:  marketplacev1alpha1.ReasonRhmRegistrationWatchkeeperEnabled,
			Message: message,
		})

		if changed {
			reqLogger.Info("Registration watchkeeper disabled status updated")

			_ = r.Client.Status().Update(context.TODO(), instance)
			r.Client.Get(context.TODO(), request.NamespacedName, instance)
		}
	}

	/******************************************************************************
	APPLY OR UPDATE RAZEE RESOURCES
	/******************************************************************************/
	razeeNamespace := &corev1.Namespace{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: *instance.Spec.TargetNamespace}, razeeNamespace)
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

	razeePrereqs := []string{}
	razeePrereqs = append(razeePrereqs, fmt.Sprintf("%v namespace", razeeNamespace.Name))

	if reflect.DeepEqual(instance.Status.RazeePrerequisitesCreated, razeePrereqs) {
		instance.Status.RazeePrerequisitesCreated = razeePrereqs
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status for razee namespace")
		}
		r.Client.Get(context.TODO(), request.NamespacedName, instance)
	}

	// apply watch-keeper-non-namespaced
	watchKeeperNonNamespace := corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.WATCH_KEEPER_NON_NAMESPACED_NAME, Namespace: *instance.Spec.TargetNamespace}, &watchKeeperNonNamespace)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)

			watchKeeperNonNamespace = *r.makeWatchKeeperNonNamespace(instance)
			if err := utils.ApplyAnnotation(&watchKeeperNonNamespace); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			err = r.Client.Create(context.TODO(), &watchKeeperNonNamespace)
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
			_ = r.Client.Status().Update(context.TODO(), instance)

			return reconcile.Result{Requeue: true}, nil
		} else {
			reqLogger.Error(err, "Failed to get resource", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.V(0).Info("Resource already exists", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)

		updatedWatchKeeperNonNameSpace := r.makeWatchKeeperNonNamespace(instance)
		patchResult, err := r.patcher.Calculate(&watchKeeperNonNamespace, updatedWatchKeeperNonNameSpace)
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
			err = r.Client.Update(context.TODO(), updatedWatchKeeperNonNameSpace)
			if err != nil {
				reqLogger.Error(err, "Failed to update resource", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No change detected on resource", "resource: ", utils.WATCH_KEEPER_NON_NAMESPACED_NAME)

	}

	razeePrereqs = append(razeePrereqs, utils.WATCH_KEEPER_NON_NAMESPACED_NAME)

	if reflect.DeepEqual(instance.Status.RazeePrerequisitesCreated, razeePrereqs) {
		instance.Status.RazeePrerequisitesCreated = razeePrereqs
		reqLogger.Info("updating status - razeeprereqs for watchkeeper non namespaced name")
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status for watchkeeper non namespaced name")
		}
		r.Client.Get(context.TODO(), request.NamespacedName, instance)
	}

	// apply watch-keeper-limit-poll config map
	watchKeeperLimitPoll := corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.WATCH_KEEPER_LIMITPOLL_NAME, Namespace: *instance.Spec.TargetNamespace}, &watchKeeperLimitPoll)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)

			watchKeeperLimitPoll = *r.makeWatchKeeperLimitPoll(instance)
			if err := utils.ApplyAnnotation(&watchKeeperLimitPoll); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			err = r.Client.Create(context.TODO(), &watchKeeperLimitPoll)
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
			_ = r.Client.Status().Update(context.TODO(), instance)

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
		patchResult, err := r.patcher.Calculate(&watchKeeperLimitPoll, updatedWatchKeeperLimitPoll)
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
			err = r.Client.Update(context.TODO(), updatedWatchKeeperLimitPoll)
			if err != nil {
				reqLogger.Error(err, "Failed to overwrite resource", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No change detected on resource", "resource: ", utils.WATCH_KEEPER_LIMITPOLL_NAME)
	}

	razeePrereqs = append(razeePrereqs, utils.WATCH_KEEPER_LIMITPOLL_NAME)

	if reflect.DeepEqual(instance.Status.RazeePrerequisitesCreated, razeePrereqs) {
		instance.Status.RazeePrerequisitesCreated = razeePrereqs
		reqLogger.Info("updating status - razeeprereqs for watchkeeper limit poll name")
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status for watchkeeper limit poll name")
		}
		r.Client.Get(context.TODO(), request.NamespacedName, instance)
	}

	// create razee-cluster-metadata
	razeeClusterMetaData := corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_CLUSTER_METADATA_NAME, Namespace: *instance.Spec.TargetNamespace}, &razeeClusterMetaData)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.RAZEE_CLUSTER_METADATA_NAME)

			razeeClusterMetaData = *r.makeRazeeClusterMetaData(instance)
			if err := utils.ApplyAnnotation(&razeeClusterMetaData); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			err = r.Client.Create(context.TODO(), &razeeClusterMetaData)
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

			_ = r.Client.Status().Update(context.TODO(), instance)
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
		patchResult, err := r.patcher.Calculate(&razeeClusterMetaData, &updatedRazeeClusterMetaData)
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
			err = r.Client.Update(context.TODO(), &updatedRazeeClusterMetaData)
			if err != nil {
				reqLogger.Error(err, "Failed to overwrite resource", "resource: ", utils.RAZEE_CLUSTER_METADATA_NAME)
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.Info("No change detected on resource", "resource: ", utils.RAZEE_CLUSTER_METADATA_NAME)
	}

	razeePrereqs = append(razeePrereqs, utils.WATCH_KEEPER_LIMITPOLL_NAME)

	if reflect.DeepEqual(instance.Status.RazeePrerequisitesCreated, razeePrereqs) {
		instance.Status.RazeePrerequisitesCreated = razeePrereqs
		reqLogger.Info("updating status- razeeprereqs for watchkeeper cluster meta data")
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status for watchkeeper cluster meta data")
		}
		r.Client.Get(context.TODO(), request.NamespacedName, instance)
	}

	// create watch-keeper-config
	watchKeeperConfig := corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.WATCH_KEEPER_CONFIG_NAME, Namespace: *instance.Spec.TargetNamespace}, &watchKeeperConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)

			watchKeeperConfig = *r.makeWatchKeeperConfigV2(instance)
			if err := utils.ApplyAnnotation(&watchKeeperConfig); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}

			if instance.Spec.ClusterDisplayName != "" {
				if watchKeeperConfig.Labels == nil {
					watchKeeperConfig.Labels = make(map[string]string)
				}

				utils.SetMapKeyValue(watchKeeperConfig.Labels, []string{"razee/cluster-metadata", "true"})
				watchKeeperConfig.Data["name"] = instance.Spec.ClusterDisplayName
			}

			err = r.Client.Create(context.TODO(), &watchKeeperConfig)
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

			_ = r.Client.Status().Update(context.TODO(), instance)

			reqLogger.Info("Resource created successfully", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
			return reconcile.Result{Requeue: true}, nil
		} else {
			reqLogger.Error(err, "Failed to get resource", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
			return reconcile.Result{}, err
		}
	}
	if err == nil {
		reqLogger.V(0).Info("Resource already exists",
			"resource", utils.WATCH_KEEPER_CONFIG_NAME,
			"uid", watchKeeperConfig.UID)

		var updatedWatchKeeperConfig v1.ConfigMap
		version, _ := watchKeeperConfig.GetAnnotations()["marketplace.redhat.com/version"]
		switch version {
		case "2":
			updatedWatchKeeperConfig = *r.makeWatchKeeperConfigV2(instance)
		case "1":
			fallthrough
		default:
			updatedWatchKeeperConfig = *r.makeWatchKeeperConfig(instance)
		}

		updatedWatchKeeperConfig.UID = watchKeeperConfig.UID
		updatedWatchKeeperConfig.ResourceVersion = watchKeeperConfig.ResourceVersion

		if !reflect.DeepEqual(updatedWatchKeeperConfig.Data, watchKeeperConfig.Data) {
			reqLogger.Info("Change detected on", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
			if err := utils.ApplyAnnotation(&updatedWatchKeeperConfig); err != nil {
				reqLogger.Error(err, "Failed to set annotation")
				return reconcile.Result{}, err
			}
			reqLogger.Info("Updating resource", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
			err = r.Client.Update(context.TODO(), &updatedWatchKeeperConfig)
			if err != nil {
				reqLogger.Error(err, "Failed to overwrite ", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
				return reconcile.Result{}, err
			}
			reqLogger.Info("Resource updated successfully", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No changed detected on resource", "resource: ", utils.WATCH_KEEPER_CONFIG_NAME)
	}

	razeePrereqs = append(razeePrereqs, utils.WATCH_KEEPER_CONFIG_NAME)

	if reflect.DeepEqual(instance.Status.RazeePrerequisitesCreated, razeePrereqs) {
		instance.Status.RazeePrerequisitesCreated = razeePrereqs
		reqLogger.Info("updating status - razeeprereqs for watchkeeper config")
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status for watchkeeper config")
		}
		r.Client.Get(context.TODO(), request.NamespacedName, instance)
	}

	// create watch-keeper-secret
	watchKeeperSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.WATCH_KEEPER_SECRET_NAME, Namespace: *instance.Spec.TargetNamespace}, &watchKeeperSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(0).Info("Resource does not exist", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
			watchKeeperSecret, err = r.makeWatchKeeperSecret(instance, request)
			if err != nil {
				return reconcile.Result{}, err
			}
			err = r.Client.Create(context.TODO(), &watchKeeperSecret)
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

			_ = r.Client.Status().Update(context.TODO(), instance)

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
			err = r.Client.Update(context.TODO(), &watchKeeperSecret)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
				return reconcile.Result{}, err
			}
			reqLogger.Info("Resource updated successfully", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No change detected on resource", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
	}

	razeePrereqs = append(razeePrereqs, utils.WATCH_KEEPER_SECRET_NAME)

	if reflect.DeepEqual(instance.Status.RazeePrerequisitesCreated, razeePrereqs) {
		instance.Status.RazeePrerequisitesCreated = razeePrereqs
		reqLogger.Info("updating status - razeeprereqs for watchkeeper secret")
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status for watchkeeper secret")
		}
		r.Client.Get(context.TODO(), request.NamespacedName, instance)
	}

	// create ibm-cos-reader-key
	ibmCosReaderKey := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.COS_READER_KEY_NAME, Namespace: *instance.Spec.TargetNamespace}, &ibmCosReaderKey)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Resource does not exist", "resource: ", utils.COS_READER_KEY_NAME)
			ibmCosReaderKey, err = r.makeCOSReaderSecret(instance, request)
			if err != nil {
				reqLogger.Error(err, "Failed to build resource", "resource: ", utils.COS_READER_KEY_NAME)
				return reconcile.Result{}, err
			}

			err = r.Client.Create(context.TODO(), &ibmCosReaderKey)
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

			_ = r.Client.Status().Update(context.TODO(), instance)

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
			err = r.Client.Update(context.TODO(), &ibmCosReaderKey)
			if err != nil {
				reqLogger.Error(err, "Failed to create resource", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
				return reconcile.Result{}, err
			}
			reqLogger.Info("Resource updated successfully", "resource: ", utils.WATCH_KEEPER_SECRET_NAME)
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.V(0).Info("No change detected on resource", "resource: ", utils.COS_READER_KEY_NAME)
	}

	razeePrereqs = append(razeePrereqs, utils.COS_READER_KEY_NAME)

	if reflect.DeepEqual(instance.Status.RazeePrerequisitesCreated, razeePrereqs) {
		instance.Status.RazeePrerequisitesCreated = razeePrereqs
		reqLogger.Info("updating status - razeeprereqs for cos reader key name")
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update status for cos reader key name")
		}
		r.Client.Get(context.TODO(), request.NamespacedName, instance)
	}

	/******************************************************************************
	Create watch-keeper deployment,rrs3-controller deployment, apply parent rrs3
	/******************************************************************************/
	reqLogger.V(0).Info("Finding Rhm RemoteResourceS3 deployment")

	if rrs3DeploymentEnabled {
		result, err := r.createOrUpdateRemoteResourceS3Deployment(instance)
		if err != nil {
			reqLogger.Error(err, "Failed to createOrUpdateRemoteResourceS3Deployment")
			return result, err
		} else if result.Requeue || result.RequeueAfter != 0 {
			return result, err
		}
	}

	if registrationEnabled {
		result, err := r.createOrUpdateWatchKeeperDeployment(instance)
		if err != nil {
			reqLogger.Error(err, "Failed to createOrUpdateWatchKeeperDeployment")
			return result, err
		} else if result.Requeue || result.RequeueAfter != 0 {
			return result, err
		}
	} else {
		reqLogger.V(0).Info("watch-keeper deployment not enabled")
	}

	depList := &appsv1.DeploymentList{}
	depListOpts := []client.ListOption{
		client.InNamespace(*instance.Spec.TargetNamespace),
	}
	err = r.Client.List(context.TODO(), depList, depListOpts...)

	var depNames []string
	for _, dep := range depList.Items {
		depNames = append(depNames, dep.Name)
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(*instance.Spec.TargetNamespace),
		client.MatchingLabels(map[string]string{
			"owned-by": "marketplace.redhat.com-razee",
		}),
	}

	err = r.Client.List(context.TODO(), podList, listOpts...)
	if err != nil {
		reqLogger.Error(err, "Failed to list deployment pods")
		return reconcile.Result{}, err
	}

	podNames := utils.GetPodNames(podList.Items)
	r.Client.Get(context.TODO(), request.NamespacedName, instance)

	if !reflect.DeepEqual(podNames, instance.Status.NodesFromRazeeDeployments) {
		instance.Status.NodesFromRazeeDeployments = podNames
		//Add NodesFromRazeeDeployments Count
		instance.Status.NodesFromRazeeDeploymentsCount = len(instance.Status.NodesFromRazeeDeployments)

		reqLogger.Info("updating status - podlist for razee deployments")
		err := r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update Status with podlist.")
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	//Only create the parent s3 resource when the razee deployment is enabled
	if rrs3DeploymentEnabled {

		var op controllerutil.OperationResult
		parentRRS3 := r.makeParentRemoteResourceS3(instance)
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, parentRRS3, func() error {
				r.updateParentRemoteResourceS3(parentRRS3, instance)
				return r.factory.SetOwnerReference(instance, parentRRS3)
			})
			return err
		})
		if err != nil {
			return reconcile.Result{}, err
		}

		reqLogger.Info(fmt.Sprintf("Resource %v successfully", op), "resource", utils.PARENT_RRS3_RESOURCE_NAME)

		if op == controllerutil.OperationResultCreated {
			message := "ParentRRS3 install finished"
			instance.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonParentRRS3Installed,
				Message: message,
			})

			_ = r.Client.Status().Update(context.TODO(), instance)
			return reconcile.Result{Requeue: true}, nil
		}

		razeePrereqs = append(razeePrereqs, utils.PARENT_RRS3_RESOURCE_NAME)

		if reflect.DeepEqual(instance.Status.RazeePrerequisitesCreated, razeePrereqs) {
			instance.Status.RazeePrerequisitesCreated = razeePrereqs
			reqLogger.Info("updating status - razeeprereqs for parent rrs3")
			err = r.Client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update status for parent rrs3")
			}
			r.Client.Get(context.TODO(), request.NamespacedName, instance)
		}
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
	err = r.Client.Get(context.Background(), client.ObjectKey{
		Name: "cluster",
	}, console)
	if err != nil {
		if !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			reqLogger.Error(err, "Failed to retrieve Console resource")
			return reconcile.Result{}, err
		}

		console = nil
	}

	if console != nil {
		reqLogger.V(0).Info("Found Console resource")
		consoleOriginalLabels := console.DeepCopy().GetLabels()
		consoleLabels := console.GetLabels()
		if consoleLabels == nil {
			consoleLabels = make(map[string]string)
		}
		consoleLabels[razeeWatchTag] = razeeWatchTagValueLite
		if !reflect.DeepEqual(consoleLabels, consoleOriginalLabels) {
			console.SetLabels(consoleLabels)
			err = r.Client.Update(context.TODO(), console)
			if err != nil {
				reqLogger.Error(err, "Failed to patch razee/watch-resource: lite label to Console resource")
				return reconcile.Result{}, err
			}
			reqLogger.Info("Patched razee/watch-resource: lite label to Console resource")
			return reconcile.Result{Requeue: true}, nil
		}
		reqLogger.V(0).Info("No patch needed on Console resource")
	}

	reqLogger.V(0).Info("finding Infrastructure resource")
	infrastructureResource := &unstructured.Unstructured{}
	infrastructureResource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Kind:    "Infrastructure",
		Version: "v1",
	})
	err = r.Client.Get(context.Background(), client.ObjectKey{
		Name: "cluster",
	}, infrastructureResource)
	if err != nil {
		if !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			reqLogger.Error(err, "Failed to retrieve Infrastructure resource")
			return reconcile.Result{}, err
		}
		infrastructureResource = nil
	}

	if infrastructureResource != nil {
		reqLogger.V(0).Info("Found Infrastructure resource")
		infrastructureOriginalLabels := infrastructureResource.DeepCopy().GetLabels()
		infrastructureLabels := infrastructureResource.GetLabels()
		if infrastructureLabels == nil {
			infrastructureLabels = make(map[string]string)
		}
		infrastructureLabels[razeeWatchTag] = razeeWatchTagValueLite
		if !reflect.DeepEqual(infrastructureLabels, infrastructureOriginalLabels) {
			infrastructureResource.SetLabels(infrastructureLabels)
			err = r.Client.Update(context.TODO(), infrastructureResource)
			if err != nil {
				reqLogger.Error(err, "Failed to patch razee/watch-resource: lite label to Infrastructure resource")
				return reconcile.Result{}, err
			}
			reqLogger.Info("Patched razee/watch-resource: lite label to Infrastructure resource")

			return reconcile.Result{Requeue: true}, nil
		}
		reqLogger.V(0).Info("No patch needed on Infrastructure resource")
	}

	reqLogger.V(0).Info("finding clusterversion resource")
	clusterVersion := &unstructured.Unstructured{}
	clusterVersion.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Kind:    "ClusterVersion",
		Version: "v1",
	})
	err = r.Client.Get(context.Background(), client.ObjectKey{
		Name: "version",
	}, clusterVersion)
	if err != nil {
		if !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			reqLogger.Error(err, "Failed to retrieve clusterversion resource")
			return reconcile.Result{}, err
		}

		clusterVersion = nil
	}

	if clusterVersion != nil {
		reqLogger.V(0).Info("Found clusterversion resource")
		clusterVersionOriginalLabels := clusterVersion.DeepCopy().GetLabels()
		clusterVersionLabels := clusterVersion.GetLabels()
		if clusterVersionLabels == nil {
			clusterVersionLabels = make(map[string]string)
		}
		clusterVersionLabels[razeeWatchTag] = razeeWatchTagValueDetail
		if !reflect.DeepEqual(clusterVersionLabels, clusterVersionOriginalLabels) {
			clusterVersion.SetLabels(clusterVersionLabels)
			err = r.Client.Update(context.TODO(), clusterVersion)
			if err != nil {
				reqLogger.Error(err, "Failed to patch razee/watch-resource: detail label to clusterversion resource")
				return reconcile.Result{}, err
			}
			reqLogger.Info("Patched razee/watch-resource: detail label to clusterversion resource")

			return reconcile.Result{Requeue: true}, nil
		}
		reqLogger.V(0).Info("No patch needed on clusterversion resource")
	}

	// check if the legacy uninstaller has run
	if instance.Spec.LegacyUninstallHasRun == nil || *instance.Spec.LegacyUninstallHasRun == false {
		r.uninstallLegacyResources(instance)
	}

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
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update final status")
			return reconcile.Result{}, err
		}
		r.Client.Get(context.TODO(), request.NamespacedName, instance)
	}

	reqLogger.Info("End of reconcile")
	return reconcile.Result{}, nil

}

// addFinalizer adds finalizers to the RazeeDeployment CR
func (r *RazeeDeploymentReconciler) addFinalizer(razee *marketplacev1alpha1.RazeeDeployment, namespace string) error {
	reqLogger := r.Log.WithValues("Request.Namespace", namespace, "Request.Name", utils.RAZEE_UNINSTALL_NAME)
	reqLogger.Info("Adding Finalizer for the razeeDeploymentFinalizer")
	razee.SetFinalizers(append(razee.GetFinalizers(), utils.RAZEE_DEPLOYMENT_FINALIZER))

	err := r.Client.Update(context.TODO(), razee)
	if err != nil {
		reqLogger.Error(err, "Failed to update RazeeDeployment with the Finalizer")
		return err
	}
	return nil
}

// Creates the razee-cluster-metadata config map and applies the TargetNamespace and the ClusterUUID stored on the Razeedeployment cr
func (r *RazeeDeploymentReconciler) makeRazeeClusterMetaData(instance *marketplacev1alpha1.RazeeDeployment) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
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
	r.factory.SetOwnerReference(instance, cm)
	return cm
}

//watch-keeper-non-namespace
func (r *RazeeDeploymentReconciler) makeWatchKeeperNonNamespace(
	instance *marketplacev1alpha1.RazeeDeployment,
) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_NON_NAMESPACED_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string]string{"v1_namespace": "true"},
	}
	r.factory.SetOwnerReference(instance, cm)
	return cm
}

//watch-keeper-non-namespace
func (r *RazeeDeploymentReconciler) makeWatchKeeperLimitPoll(
	instance *marketplacev1alpha1.RazeeDeployment,
) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_LIMITPOLL_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
	}
	r.factory.SetOwnerReference(instance, cm)
	return cm
}

// Creates watchkeeper config and applies the razee-dash-url stored on the Razeedeployment cr
func (r *RazeeDeploymentReconciler) makeWatchKeeperConfig(instance *marketplacev1alpha1.RazeeDeployment) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_CONFIG_NAME,
			Namespace: *instance.Spec.TargetNamespace,
			Annotations: map[string]string{
				"marketplace.redhat.com/version": "1",
			},
		},
		Data: map[string]string{
			"RAZEEDASH_URL":   instance.Spec.DeployConfig.RazeeDashUrl,
			"START_DELAY_MAX": "0",
		},
	}
	r.factory.SetOwnerReference(instance, cm)
	return cm
}

// Creates watchkeeper config and applies the razee-dash-url stored on the Razeedeployment cr
func (r *RazeeDeploymentReconciler) makeWatchKeeperConfigV2(
	instance *marketplacev1alpha1.RazeeDeployment,
) *corev1.ConfigMap {
	data := map[string]string{
		"RAZEEDASH_URL":       instance.Spec.DeployConfig.RazeeDashUrl,
		"START_DELAY_MAX":     "0",
		"CLUSTER_ID_OVERRIDE": instance.Spec.ClusterUUID,
	}

	if instance.Spec.ClusterDisplayName != "" {
		data["DEFAULT_CLUSTER_NAME"] = instance.Spec.ClusterDisplayName
		data["name"] = instance.Spec.ClusterDisplayName
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_CONFIG_NAME,
			Namespace: *instance.Spec.TargetNamespace,
			Annotations: map[string]string{
				"marketplace.redhat.com/version": "2",
			},
		},
		Data: data,
	}
	r.factory.SetOwnerReference(instance, cm)
	return cm
}

// GetDataFromRhmSecret Uses the SecretKeySelector struct to to retrieve byte data from a specified key
func (r *RazeeDeploymentReconciler) GetDataFromRhmSecret(request reconcile.Request, sel corev1.SecretKeySelector) ([]byte, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "request.Name", request.Name)
	reqLogger.Info("Beginning of rhm-operator-secret reconcile")

	rhmOperatorSecret := corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
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
func (r *RazeeDeploymentReconciler) makeWatchKeeperSecret(instance *marketplacev1alpha1.RazeeDeployment, request reconcile.Request) (corev1.Secret, error) {
	selector := instance.Spec.DeployConfig.RazeeDashOrgKey
	key, err := r.GetDataFromRhmSecret(request, *selector)

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_SECRET_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string][]byte{"RAZEEDASH_ORG_KEY": key},
	}
	r.factory.SetOwnerReference(instance, &secret)
	return secret, err
}

// Creates the rhm-cos-reader-key and applies the ibm-cos-reader-key from rhm-operator-secret using the selector stored on the Razeedeployment cr
func (r *RazeeDeploymentReconciler) makeCOSReaderSecret(instance *marketplacev1alpha1.RazeeDeployment, request reconcile.Request) (corev1.Secret, error) {
	selector := instance.Spec.DeployConfig.IbmCosReaderKey
	key, err := r.GetDataFromRhmSecret(request, *selector)

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.COS_READER_KEY_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string][]byte{"accesskey": []byte(key)},
	}

	r.factory.SetOwnerReference(instance, &secret)
	return secret, err
}

// Creates the "parent" RemoteResourceS3 and applies the name of the cos-reader-key and ChildUrl constructed during reconciliation of the rhm-operator-secret
func (r *RazeeDeploymentReconciler) makeParentRemoteResourceS3(
	instance *marketplacev1alpha1.RazeeDeployment) *marketplacev1alpha1.RemoteResourceS3 {

	return r.updateParentRemoteResourceS3(&marketplacev1alpha1.RemoteResourceS3{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.PARENT_RRS3_RESOURCE_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
	}, instance)

	/*
		return &marketplacev1alpha1.RemoteResourceS3{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.PARENT_RRS3_RESOURCE_NAME,
				Namespace: *instance.Spec.TargetNamespace,
			},
			Spec: marketplacev1alpha1.RemoteResourceS3Spec{
				Auth: marketplacev1alpha1.Auth{
					Iam: &marketplacev1alpha1.Iam{
						ResponseType: "cloud_iam",
						GrantType:    "urn:ibm:params:oauth:grant-type:apikey",
						URL:          "https://iam.cloud.ibm.com/identity/token",
						APIKeyRef: marketplacev1alpha1.APIKeyRef{
							ValueFrom: marketplacev1alpha1.ValueFrom{
								SecretKeyRef: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: utils.COS_READER_KEY_NAME,
									},
									Key: "accesskey",
								},
							},
						},
					},
				},
				Requests: []marketplacev1alpha1.Request{
					{
						Options: marketplacev1alpha1.S3Options{
							URL: *instance.Spec.ChildUrl,
						},
					},
				},
			},
		}
	*/
}

func (r *RazeeDeploymentReconciler) updateParentRemoteResourceS3(parentRRS3 *marketplacev1alpha1.RemoteResourceS3, instance *marketplacev1alpha1.RazeeDeployment) *marketplacev1alpha1.RemoteResourceS3 {

	parentRRS3.Spec = marketplacev1alpha1.RemoteResourceS3Spec{
		Auth: marketplacev1alpha1.Auth{
			Iam: &marketplacev1alpha1.Iam{
				ResponseType: "cloud_iam",
				GrantType:    "urn:ibm:params:oauth:grant-type:apikey",
				URL:          "https://iam.cloud.ibm.com/identity/token",
				APIKeyRef: marketplacev1alpha1.APIKeyRef{
					ValueFrom: marketplacev1alpha1.ValueFrom{
						SecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: utils.COS_READER_KEY_NAME,
							},
							Key: "accesskey",
						},
					},
				},
			},
		},
		Requests: []marketplacev1alpha1.Request{
			{
				Options: marketplacev1alpha1.S3Options{
					URL: *instance.Spec.ChildUrl,
				},
			},
		},
	}

	return parentRRS3
}

//Undeploy the razee deployment and parent
func (r *RazeeDeploymentReconciler) removeRazeeDeployments(
	req *marketplacev1alpha1.RazeeDeployment,
) error {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("removing razee deployment resources: childRRS3, parentRRS3, RRS3 deployment")

	maxRetry := 3

	childRRS3 := marketplacev1alpha1.RemoteResourceS3{}
	err := utils.Retry(func() error {
		reqLogger.Info("Listing childRRS3")

		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "child", Namespace: *req.Spec.TargetNamespace}, &childRRS3)
		if err != nil && !errors.IsNotFound((err)) {
			reqLogger.Error(err, "could not get resource", "Kind", "RemoteResourceS3")
			return err
		}

		if err != nil && errors.IsNotFound((err)) {
			reqLogger.Info("ChildRRS3 deleted")
			return nil
		}

		err = r.Client.Delete(context.TODO(), &childRRS3)
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "could not delete childRRS3")
			return err
		}

		return fmt.Errorf("error on deletion of childRRS3 %d: %w", maxRetry, utils.ErrMaxRetryExceeded)

	}, maxRetry)

	if golangerrors.Is(err, utils.ErrMaxRetryExceeded) {
		reqLogger.Info("retry limit exceeded, removing finalizers on childRRS3", "err", err.Error())

		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			key, _ := client.ObjectKeyFromObject(&childRRS3)

			err := r.Client.Get(context.TODO(), key, &childRRS3)
			if err != nil {
				return err
			}

			if utils.Contains(childRRS3.GetFinalizers(), utils.RRS3_FINALIZER) {
				childRRS3.SetFinalizers(utils.RemoveKey(childRRS3.GetFinalizers(), utils.CONTROLLER_FINALIZER))
			}

			return r.Client.Update(context.TODO(), &childRRS3)
		})

		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "error updating childRRS3 finalizers")
			return err
		}

		if errors.IsNotFound(err) {
			reqLogger.Info("removed finalizers on child rrs3")
		}
	}

	parentRRS3 := marketplacev1alpha1.RemoteResourceS3{}
	err = utils.Retry(func() error {
		reqLogger.Info("Listing parentRRS3")

		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.PARENT_RRS3_RESOURCE_NAME, Namespace: *req.Spec.TargetNamespace}, &parentRRS3)
		if err != nil && !errors.IsNotFound((err)) {
			reqLogger.Error(err, "could not get resource", "Kind", "RemoteResourceS3")
			return err
		}

		if err != nil && errors.IsNotFound((err)) {
			reqLogger.Info("ParentRRS3 deleted")
			return nil
		}

		err = r.Client.Delete(context.TODO(), &parentRRS3)
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "could not delete parentRRS3")
			return err
		}

		return fmt.Errorf("error on deletion of parentRRS3 %d: %w", maxRetry, utils.ErrMaxRetryExceeded)

	}, maxRetry)

	if golangerrors.Is(err, utils.ErrMaxRetryExceeded) {
		reqLogger.Info("retry limit exceeded, removing finalizers on parentRRS3", "err", err.Error())

		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			key, _ := client.ObjectKeyFromObject(&parentRRS3)

			err := r.Client.Get(context.TODO(), key, &parentRRS3)
			if err != nil {
				return err
			}

			if utils.Contains(parentRRS3.GetFinalizers(), utils.RRS3_FINALIZER) {
				parentRRS3.SetFinalizers(utils.RemoveKey(parentRRS3.GetFinalizers(), utils.RRS3_FINALIZER))
			}

			return r.Client.Update(context.TODO(), &parentRRS3)
		})

		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "error updating updatingRRS3 finalizers")
			return err
		}

		if errors.IsNotFound(err) {
			reqLogger.Info("removed finlizers on parent rrs3")
		}
	}

	//Delete the deployment
	err = utils.Retry(func() error {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME,
				Namespace: *req.Spec.TargetNamespace,
			},
		}

		reqLogger.Info("deleting deployment", "name", utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME)
		err = r.Client.Delete(context.TODO(), deployment)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info("rrs3 deployment deleted", "name", utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME)
				return nil
			}

			reqLogger.Error(err, "could not delete deployment", "name", utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME)
			return err
		}

		return fmt.Errorf("error on deletion of rrs3 deployment %d: %w", maxRetry, utils.ErrMaxRetryExceeded)
	}, maxRetry)

	if err != nil && !golangerrors.Is(err, utils.ErrMaxRetryExceeded) {
		reqLogger.Error(err, "error deleting rrs3 deployment resources")
	}

	return nil
}

//Undeploy the watchkeeper deployment
func (r *RazeeDeploymentReconciler) removeWatchkeeperDeployment(req *marketplacev1alpha1.RazeeDeployment) (*reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Starting delete of watchkeeper deployment")

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME,
			Namespace: *req.Spec.TargetNamespace,
		},
	}
	reqLogger.Info("deleting deployment", "name", utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME)
	err := r.Client.Delete(context.TODO(), deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watchkeeper not found, deployment already deleted")
			return nil, nil
		}
		reqLogger.Error(err, "could not delete deployment", "name", utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME)
	}
	//deployment deleted - requeue
	return &reconcile.Result{Requeue: true}, nil

}

// fullUninstall deletes resources created by razee deployment
func (r *RazeeDeploymentReconciler) fullUninstall(
	req *marketplacev1alpha1.RazeeDeployment,
) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Starting full uninstall of razee resources")

	if req.Spec.TargetNamespace == nil {
		if req.Status.RazeeJobInstall != nil {
			req.Spec.TargetNamespace = &req.Status.RazeeJobInstall.RazeeNamespace
		} else {
			req.Spec.TargetNamespace = &req.Namespace
		}
	}

	//Remove razee deployments and reconcile if requested
	err := r.removeRazeeDeployments(req)
	if err != nil {
		return reconcile.Result{}, err
	}

	configMaps := []string{
		utils.WATCH_KEEPER_CONFIG_NAME,
		utils.RAZEE_CLUSTER_METADATA_NAME,
		utils.WATCH_KEEPER_LIMITPOLL_NAME,
		utils.WATCH_KEEPER_NON_NAMESPACED_NAME,
		"clustersubscription",
	}
	for _, configMapName := range configMaps {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: *req.Spec.TargetNamespace,
			},
		}
		reqLogger.Info("deleting configmap", "name", configMapName)
		err = r.Client.Delete(context.TODO(), configMap)
		if err != nil && !errors.IsNotFound((err)) {
			reqLogger.Error(err, "could not delete configmap", "name", configMapName)
		}
	}

	secrets := []string{
		utils.COS_READER_KEY_NAME,
		utils.WATCH_KEEPER_SECRET_NAME,
		"clustersubscription",
	}
	for _, secretName := range secrets {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: *req.Spec.TargetNamespace,
			},
		}
		reqLogger.Info("deleting secret", "name", secretName)
		err = r.Client.Delete(context.TODO(), secret)
		if err != nil && !errors.IsNotFound((err)) {
			reqLogger.Error(err, "could not delete secret", "name", secretName)
		}
	}

	//remove the watchkeeper deployment
	r.removeWatchkeeperDeployment(req)

	reqLogger.Info("Removing finalizers on razee cr")

	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), utils.RAZEE_DEPLOYMENT_FINALIZER))
	err = r.Client.Update(context.TODO(), req)
	if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("Full uninstall of razee is complete")
	return reconcile.Result{}, nil
}

//uninstallLegacyResources deletes resources used by version 1.3 of the operator and below.
func (r *RazeeDeploymentReconciler) uninstallLegacyResources(
	req *marketplacev1alpha1.RazeeDeployment,
) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Starting legacy uninstall")

	deletePolicy := metav1.DeletePropagationForeground

	foundJob := batch.Job{}
	jobName := types.NamespacedName{
		Name:      utils.RAZEE_DEPLOY_JOB_NAME,
		Namespace: req.Namespace,
	}
	reqLogger.Info("finding legacy install job", "name", jobName)
	err := r.Client.Get(context.TODO(), jobName, &foundJob)
	if err == nil || errors.IsNotFound(err) {
		reqLogger.Info("cleaning up install job")
		err = r.Client.Delete(context.TODO(), &foundJob, client.PropagationPolicy(deletePolicy))
		if err != nil && !errors.IsNotFound(err) && err.Error() != "resource name may not be empty" {
			reqLogger.Error(err, "cleaning up install job failed")
		}

	}

	customResourceKinds := []string{
		"RemoteResourceS3",
		"RemoteResource",
		"FeatureFlagSetLD",
		"ManagedSet",
		"MustacheTemplate",
		"RemoteResourceS3Decrypt",
	}

	reqLogger.Info("Deleting legacy custom resources")
	for _, customResourceKind := range customResourceKinds {
		customResourceList := &unstructured.UnstructuredList{}
		customResourceList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "deploy.razee.io",
			Kind:    customResourceKind,
			Version: "v1alpha2",
		})

		// get custom resources for each crd
		reqLogger.Info("Listing legacy custom resources", "Kind", customResourceKind)
		err = r.Client.List(context.TODO(), customResourceList, client.InNamespace(*req.Spec.TargetNamespace))
		if err != nil && !errors.IsNotFound(err) && err.Error() != fmt.Sprintf("no matches for kind %q in version %q", customResourceKind, "deploy.razee.io/v1alpha2") {
			reqLogger.Error(err, "could not list custom resources", "Kind", customResourceKind)
		}

		if err != nil && err.Error() == fmt.Sprintf("no matches for kind %q in version %q", customResourceKind, "deploy.razee.io/v1alpha2") {
			reqLogger.Info("No legacy custom resource found", "Resource Kind", customResourceKind)
		}

		if err == nil {
			for _, cr := range customResourceList.Items {
				reqLogger.Info("Deleteing custom resource", "custom resource", cr)
				err := r.Client.Delete(context.TODO(), &cr)
				if err != nil && !errors.IsNotFound(err) {
					reqLogger.Error(err, "could not delete custom resource", "custom resource", cr)
				}
			}
		}
	}

	// sleep 5 seconds to let custom resource deletion complete
	time.Sleep(time.Second * 5)
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
		reqLogger.Info("deleting legacy service account", "name", saName)
		err = r.Client.Delete(context.TODO(), serviceAccount, client.PropagationPolicy(deletePolicy))
		if err != nil && !errors.IsNotFound((err)) {
			reqLogger.Error(err, "could not delete service account", "name", saName)
		}
	}

	clusterroles := []string{
		"razeedeploy-admin-cr",
		"redhat-marketplace-razeedeploy",
	}

	for _, clusterRoleNames := range clusterroles {
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterRoleNames,
				Namespace: *req.Spec.TargetNamespace,
			},
		}
		reqLogger.Info("deleting legacy cluster role", "name", clusterRoleNames)
		err = r.Client.Delete(context.TODO(), clusterRole, client.PropagationPolicy(deletePolicy))
		if err != nil && !errors.IsNotFound((err)) {
			reqLogger.Error(err, "could not delete cluster role", "name", clusterRoleNames)
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
		reqLogger.Info("deleting legacy deployment", "name", deploymentName)
		err = r.Client.Delete(context.TODO(), deployment, client.PropagationPolicy(deletePolicy))
		if err != nil && !errors.IsNotFound((err)) {
			reqLogger.Error(err, "could not delete deployment", "name", deploymentName)
		}
	}

	req.Spec.LegacyUninstallHasRun = ptr.Bool(true)
	err = r.Client.Update(context.TODO(), req)
	if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("Legacy uninstall complete")
	return reconcile.Result{}, nil
}

func (r *RazeeDeploymentReconciler) createOrUpdateRemoteResourceS3Deployment(
	instance *marketplacev1alpha1.RazeeDeployment,
) (reconcile.Result, error) {
	rrs3Deployment, err := r.factory.NewRemoteResourceS3Deployment()

	if err != nil {
		return reconcile.Result{}, err
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, rrs3Deployment, func() error {
			r.factory.SetControllerReference(instance, rrs3Deployment)
			return r.factory.UpdateRemoteResourceS3Deployment(rrs3Deployment)
		})
		return err
	})

	if err != nil {
		return reconcile.Result{}, err
	}

	if instance.Status.Conditions.SetCondition(status.Condition{
		Type:    marketplacev1alpha1.ConditionDeploymentEnabled,
		Status:  corev1.ConditionTrue,
		Reason:  marketplacev1alpha1.ReasonRhmRemoteResourceS3DeploymentEnabled,
		Message: "RemoteResourceS3 deployment enabled",
	}) {
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return r.Client.Status().Update(context.TODO(), instance)
		})
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *RazeeDeploymentReconciler) createOrUpdateWatchKeeperDeployment(
	instance *marketplacev1alpha1.RazeeDeployment,
) (reconcile.Result, error) {
	watchKeeperDeployment, err := r.factory.NewWatchKeeperDeployment()

	if err != nil {
		return reconcile.Result{}, err
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, watchKeeperDeployment, func() error {
			r.factory.SetControllerReference(instance, watchKeeperDeployment)
			return r.factory.UpdateWatchKeeperDeployment(watchKeeperDeployment)
		})
		return err
	})

	if err != nil {
		return reconcile.Result{}, err
	}

	if instance.Status.Conditions.SetCondition(status.Condition{
		Type:    marketplacev1alpha1.ConditionRegistrationEnabled,
		Status:  corev1.ConditionTrue,
		Reason:  marketplacev1alpha1.ReasonRhmRegistrationWatchkeeperEnabled,
		Message: "Registration deployment enabled",
	}) {
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return r.Client.Status().Update(context.TODO(), instance)
		})
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}
