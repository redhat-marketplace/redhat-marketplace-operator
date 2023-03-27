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
	"bytes"
	"context"
	"fmt"
	"reflect"
	"time"

	golangerrors "errors"

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	"golang.org/x/time/rate"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// blank assignment to verify that ReconcileRazeeDeployment implements reconcile.Reconciler
var _ reconcile.Reconciler = &RazeeDeploymentReconciler{}

// RazeeDeploymentReconciler reconciles a RazeeDeployment object
type RazeeDeploymentReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client  client.Client
	Scheme  *runtime.Scheme
	Log     logr.Logger
	cfg     *config.OperatorConfig
	factory *manifests.Factory
}

func (r *RazeeDeploymentReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
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
	mapFn := handler.MapFunc(
		func(obj client.Object) []reconcile.Request {
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name:      utils.RAZEE_NAME,
					Namespace: obj.GetNamespace(),
				}},
			}
		})

	// watch keeper configmaps
	cmp := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew.GetName() == utils.WATCH_KEEPER_NON_NAMESPACED_NAME || e.ObjectNew.GetName() == utils.WATCH_KEEPER_CONFIG_NAME || e.ObjectNew.GetName() == utils.WATCH_KEEPER_LIMITPOLL_NAME {
				return e.ObjectOld != e.ObjectNew
			}

			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object.GetName() == utils.WATCH_KEEPER_NON_NAMESPACED_NAME || e.Object.GetName() == utils.WATCH_KEEPER_CONFIG_NAME || e.Object.GetName() == utils.WATCH_KEEPER_LIMITPOLL_NAME {
				return true
			}

			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Object.GetName() == utils.WATCH_KEEPER_NON_NAMESPACED_NAME || e.Object.GetName() == utils.WATCH_KEEPER_CONFIG_NAME || e.Object.GetName() == utils.WATCH_KEEPER_LIMITPOLL_NAME {
				return true
			}

			return false
		},
	}

	// Find secret
	p := predicate.Funcs{
		// Ensures RazeeDeployment is only reconciled for appropriate Secrets
		// And not any secrets, regardless of namespace

		UpdateFunc: func(e event.UpdateEvent) bool {
			label, _ := utils.GetMapKeyValue(utils.LABEL_RHM_OPERATOR_WATCH)
			// The object doesn't contain label "foo", so the event will be
			// ignored.
			if _, ok := e.ObjectOld.GetLabels()[label]; !ok {
				return false
			}

			return e.ObjectOld != e.ObjectNew
		},
		CreateFunc: func(e event.CreateEvent) bool {
			label, _ := utils.GetMapKeyValue(utils.LABEL_RHM_OPERATOR_WATCH)

			if e.Object.GetName() == utils.RHM_OPERATOR_SECRET_NAME {
				return true
			}

			if _, ok := e.Object.GetLabels()[label]; !ok {
				return false
			}

			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			label, _ := utils.GetMapKeyValue(utils.LABEL_RHM_OPERATOR_WATCH)

			if _, ok := e.Object.GetLabels()[label]; !ok {
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
			return e.Object.GetLabels()["owned-by"] == "marketplace.redhat.com-razee"
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetLabels()["owned-by"] == "marketplace.redhat.com-razee"
		},
	}

	// Create a new controller
	return ctrl.NewControllerManagedBy(mgr).
		// Should be covered by cache filter
		// WithEventFilter(predicates.NamespacePredicate(r.cfg.DeployedNamespace)).
		For(&marketplacev1alpha1.RazeeDeployment{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			Reconciler: r,
			RateLimiter: workqueue.NewMaxOfRateLimiter(
				workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1000*time.Second),
				&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 300)},
			),
		}).
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &marketplacev1alpha1.RazeeDeployment{},
		}).
		Watches(&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(p)).
		Watches(&source.Kind{Type: &corev1.Pod{}},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(pp)).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(cmp)).
		Watches(
			&source.Kind{Type: &marketplacev1alpha1.RemoteResourceS3{}},
			&handler.EnqueueRequestForOwner{
				OwnerType: &marketplacev1alpha1.RazeeDeployment{},
			},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace=system,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=update;patch;delete,resourceNames=watch-keeper-non-namespaced;watch-keeper-limit-poll;razee-cluster-metadata;watch-keeper-config
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=update;patch;delete,resourceNames=rhm-operator-secret;watch-keeper-secret;clustersubscription;rhm-cos-reader-key
// +kubebuilder:rbac:groups=apps,namespace=system,resources=deployments;deployments/finalizers,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=apps,namespace=system,resources=deployments;deployments/finalizers,verbs=update;patch;delete,resourceNames=rhm-remoteresources3-controller;rhm-watch-keeper
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=razeedeployments;razeedeployments/finalizers;razeedeployments/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=remoteresources3s,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=remoteresources3s,verbs=update;patch;delete,resourceNames=child;parent
// +kubebuilder:rbac:groups="operators.coreos.com",resources=catalogsources,verbs=create;get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=catalogsources,verbs=delete,resourceNames=ibm-operator-catalog;opencloud-operators

// operator_config
// +kubebuilder:rbac:groups="config.openshift.io",resources=clusterversions,verbs=get;list;watch

// Reconcile reads that state of the cluster for a RazeeDeployment object and makes changes based on the state read
// and what is in the RazeeDeployment.Spec
func (r *RazeeDeploymentReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RazeeDeployment")

	// Fetch the RazeeDeployment instance
	instance := &marketplacev1alpha1.RazeeDeployment{}
	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
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

	// Remove finalizer used by previous versions, ownerref gc deletion is used for cleanup
	if controllerutil.ContainsFinalizer(instance, utils.RAZEE_DEPLOYMENT_FINALIZER) {
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			controllerutil.RemoveFinalizer(instance, utils.RAZEE_DEPLOYMENT_FINALIZER)
			return r.Client.Update(context.TODO(), instance)
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	// This has no watch
	for _, catalogSrcName := range [2]string{utils.IBM_CATALOGSRC_NAME, utils.OPENCLOUD_CATALOGSRC_NAME} {
		if result, err := r.createCatalogSource(instance, catalogSrcName); err != nil {
			return result, err
		}
	}

	// if not enabled then exit
	if !instance.Spec.Enabled {
		reqLogger.Info("Razee not enabled")

		if err := r.removeWatchkeeperDeployment(instance); err != nil {
			return reconcile.Result{}, err
		}

		if err := r.removeRazeeDeployments(instance); err != nil {
			return reconcile.Result{}, err
		}

		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionRazeeNotEnabled) {
				return r.Client.Status().Update(context.TODO(), instance)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// check name match
	if instance.Name != utils.RAZEE_NAME {
		reqLogger.Info("Names other than the default are not supported",
			"supportedName", utils.RAZEE_DEPLOY_JOB_NAME,
			"name", instance.Name,
		)

		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionRazeeNameMismatch) {
				return r.Client.Status().Update(context.TODO(), instance)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// Set install start condition
	if instance.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionInstalling) == nil {
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionRazeeStartInstall) {
				return r.Client.Status().Update(context.TODO(), instance)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	// set targetnamespace if not set
	if instance.Spec.TargetNamespace == nil {
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}

			if instance.Status.RazeeJobInstall != nil {
				instance.Spec.TargetNamespace = &instance.Status.RazeeJobInstall.RazeeNamespace
			} else {
				instance.Spec.TargetNamespace = &instance.Namespace
			}
			return r.Client.Update(context.TODO(), instance)

		}); err != nil {
			return reconcile.Result{}, err
		}

		reqLogger.Info("set target namespace to", "namespace", instance.Spec.TargetNamespace)
	}

	// nil deprecated status fields
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
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

		return r.Client.Update(context.TODO(), instance)
	}); err != nil {
		return reconcile.Result{}, err
	}

	// set watch label on rhm-operator-secret
	secretName := utils.RHM_OPERATOR_SECRET_NAME

	if instance.Spec.DeploySecretName != nil {
		secretName = *instance.Spec.DeploySecretName
	}

	rhmOperatorSecret := &corev1.Secret{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      secretName,
		Namespace: request.Namespace,
	}, rhmOperatorSecret); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Failed to find rhm-operator-secret")
			// nothing to do, secret watch will trigger reconciler when rhm-operator-secret is created
			return reconcile.Result{}, nil
		} else {
			return reconcile.Result{}, err
		}
	}

	if !utils.HasMapKey(rhmOperatorSecret.ObjectMeta.Labels, utils.LABEL_RHM_OPERATOR_WATCH) {
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      secretName,
				Namespace: request.Namespace,
			}, rhmOperatorSecret); err != nil {
				return err
			}

			if rhmOperatorSecret.ObjectMeta.Labels == nil {
				rhmOperatorSecret.ObjectMeta.Labels = make(map[string]string)
			}

			utils.SetMapKeyValue(rhmOperatorSecret.ObjectMeta.Labels, utils.LABEL_RHM_OPERATOR_WATCH)

			return r.Client.Update(context.TODO(), rhmOperatorSecret)
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	//update deployconfig
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}

		// AddSecretFieldsToStruct updates instance, make a copy since we need to compare, consider rewriting
		instanceCopy := instance.DeepCopy()
		razeeConfigurationValues, missingItems, err := utils.AddSecretFieldsToStruct(rhmOperatorSecret.Data, *instanceCopy)
		if err != nil {
			return err
		}

		if !utils.StringSliceEqual(instance.Status.MissingDeploySecretValues, missingItems) ||
			!reflect.DeepEqual(instance.Spec.DeployConfig, &razeeConfigurationValues) {
			instance.Status.MissingDeploySecretValues = missingItems
			instance.Spec.DeployConfig = &razeeConfigurationValues

			return r.Client.Update(context.TODO(), instance)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		return reconcile.Result{}, err
	}

	if len(instance.Status.MissingDeploySecretValues) > 0 {
		reqLogger.Info("Missing required razee configuration values, will wait until the secret is updated")
		return reconcile.Result{}, nil
	}

	reqLogger.V(0).Info("all secret values found")

	//construct the childURL
	url := fmt.Sprintf("%s/%s/%s/%s", instance.Spec.DeployConfig.IbmCosURL, instance.Spec.DeployConfig.BucketName, instance.Spec.ClusterUUID, instance.Spec.DeployConfig.ChildRSS3FIleName)
	if instance.Spec.ChildUrl == nil || (instance.Spec.ChildUrl != nil && *instance.Spec.ChildUrl != url) {
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			instance.Spec.ChildUrl = &url
			return r.Client.Update(context.TODO(), instance)
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	reqLogger.V(0).Info("All required razee configuration values have been found")

	// Check if the RazeeDeployment is disabled, in this case remove the razee deployment and parents3
	rrs3DeploymentEnabled := instance.Spec.Features == nil || instance.Spec.Features.Deployment == nil || *instance.Spec.Features.Deployment
	if !rrs3DeploymentEnabled {
		//razee deployment disabled - if the deployment was found, delete it
		if err := r.removeRazeeDeployments(instance); err != nil {
			return reconcile.Result{}, err
		}

		//Deployment is disabled - update status
		reqLogger.V(0).Info("RemoteResourceS3 deployment is disabled")
		//update status to reflect disabled
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionResourceS3DeploymentDisabled) {
				return r.Client.Status().Update(context.TODO(), instance)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
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
		if err := r.removeWatchkeeperDeployment(instance); err != nil {
			return reconcile.Result{}, err
		}

		//Deployment is disabled - update status
		reqLogger.V(0).Info("Registration watchkeeper deployment is disabled")
		//update status to reflect disabled
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionRhmRegistrationWatchkeeperDisabled) {
				return r.Client.Status().Update(context.TODO(), instance)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	/******************************************************************************
	APPLY OR UPDATE RAZEE RESOURCES
	/******************************************************************************/
	razeeNamespace := &corev1.Namespace{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: *instance.Spec.TargetNamespace}, razeeNamespace); err != nil {
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

	razeePrereqs := []string{}
	razeePrereqs = append(razeePrereqs, fmt.Sprintf("%v namespace", razeeNamespace.Name))
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if !reflect.DeepEqual(instance.Status.RazeePrerequisitesCreated, razeePrereqs) {
			instance.Status.RazeePrerequisitesCreated = razeePrereqs
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// apply watch-keeper-non-namespaced
	if err := r.factory.CreateOrUpdate(r.Client, nil, func() (client.Object, error) {
		return r.makeWatchKeeperNonNamespace(instance), nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionWatchKeeperNonNamespacedInstalled) {
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// apply watch-keeper-limit-poll config map
	if err := r.factory.CreateOrUpdate(r.Client, nil, func() (client.Object, error) {
		return r.makeWatchKeeperLimitPoll(instance), nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionWatchKeeperLimitPollInstalled) {
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// create razee-cluster-metadata
	if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
		return r.makeRazeeClusterMetaData(instance), nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionRazeeClusterMetaDataInstalled) {
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// create watch-keeper-config
	if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
		return r.makeWatchKeeperConfigV2(instance), nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionWatchKeeperConfigInstalled) {
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// create watch-keeper-secret
	if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
		secret, err := r.makeWatchKeeperSecret(instance, request)
		return &secret, err
	}); err != nil {
		return reconcile.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionWatchKeeperSecretInstalled) {
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// create ibm-cos-reader-key
	if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
		secret, err := r.makeCOSReaderSecret(instance, request)
		return &secret, err
	}); err != nil {
		return reconcile.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionCosReaderKeyInstalled) {
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	/******************************************************************************
	Create watch-keeper deployment,rrs3-controller deployment, apply parent rrs3
	/******************************************************************************/
	reqLogger.V(0).Info("Finding Rhm RemoteResourceS3 deployment")

	if rrs3DeploymentEnabled {
		if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
			dep, err := r.factory.NewRemoteResourceS3Deployment()
			return dep, err
		}); err != nil {
			return reconcile.Result{}, err
		}

		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionRhmRemoteResourceS3DeploymentEnabled) {
				return r.Client.Status().Update(context.TODO(), instance)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	if registrationEnabled {
		if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
			dep, err := r.factory.NewWatchKeeperDeployment()
			return dep, err
		}); err != nil {
			return reconcile.Result{}, err
		}

		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionRhmRegistrationWatchkeeperEnabled) {
				return r.Client.Status().Update(context.TODO(), instance)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	depList := &appsv1.DeploymentList{}
	depListOpts := []client.ListOption{
		client.InNamespace(*instance.Spec.TargetNamespace),
	}
	if err := r.Client.List(context.TODO(), depList, depListOpts...); err != nil {
		return reconcile.Result{}, err
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(*instance.Spec.TargetNamespace),
		client.MatchingLabels(map[string]string{
			"owned-by": "marketplace.redhat.com-razee",
		}),
	}

	if err := r.Client.List(context.TODO(), podList, listOpts...); err != nil {
		reqLogger.Error(err, "Failed to list deployment pods")
		return reconcile.Result{}, err
	}

	podNames := utils.GetPodNames(podList.Items)

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if !reflect.DeepEqual(podNames, instance.Status.NodesFromRazeeDeployments) {
			instance.Status.NodesFromRazeeDeployments = podNames
			//Add NodesFromRazeeDeployments Count
			instance.Status.NodesFromRazeeDeploymentsCount = len(instance.Status.NodesFromRazeeDeployments)

			reqLogger.Info("updating status - podlist for razee deployments")
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	//Only create the parent s3 resource when the razee deployment is enabled
	if rrs3DeploymentEnabled {
		// Set the remoteresources3-controller as the controller, since it owns the finalizer
		rrs3Deployment := &appsv1.Deployment{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME,
			Namespace: request.Namespace,
		}, rrs3Deployment)
		if errors.IsNotFound(err) {
			return reconcile.Result{RequeueAfter: time.Second * 60}, nil
		} else if err != nil {
			return reconcile.Result{}, err
		}

		if err := r.factory.CreateOrUpdate(r.Client, rrs3Deployment, func() (client.Object, error) {
			return r.makeParentRemoteResourceS3(instance), nil
		}); err != nil {
			return reconcile.Result{}, err
		}

		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionParentRRS3Installed) {
				return r.Client.Status().Update(context.TODO(), instance)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}

		razeePrereqs = append(razeePrereqs, utils.PARENT_RRS3_RESOURCE_NAME)
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			if !reflect.DeepEqual(instance.Status.RazeePrerequisitesCreated, razeePrereqs) {
				instance.Status.RazeePrerequisitesCreated = razeePrereqs
				return r.Client.Status().Update(context.TODO(), instance)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Complete Status
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionRazeeInstallFinished) ||
			instance.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionRazeeInstallComplete) {
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("End of reconcile")
	return reconcile.Result{}, nil
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

// watch-keeper-non-namespace
func (r *RazeeDeploymentReconciler) makeWatchKeeperNonNamespace(
	instance *marketplacev1alpha1.RazeeDeployment,
) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_NON_NAMESPACED_NAME,
			Namespace: *instance.Spec.TargetNamespace,
		},
		Data: map[string]string{"v1_namespace": "lite", "v1_node": "lite", "config.openshift.io_v1_clusterversion": "lite", "config.openshift.io_v1_infrastructure": "lite", "config.openshift.io_v1_console": "lite"},
	}
	r.factory.SetOwnerReference(instance, cm)
	return cm
}

// watch-keeper-non-namespace
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
			Labels: map[string]string{
				"razee/cluster-metadata": "true",
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

// Undeploy the razee deployment and parent
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
			key := client.ObjectKeyFromObject(&childRRS3)

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
			key := client.ObjectKeyFromObject(&parentRRS3)

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

// Undeploy the watchkeeper deployment
func (r *RazeeDeploymentReconciler) removeWatchkeeperDeployment(req *marketplacev1alpha1.RazeeDeployment) error {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Starting delete of watchkeeper deployment")

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME,
			Namespace: *req.Spec.TargetNamespace,
		},
	}
	reqLogger.Info("deleting deployment", "name", utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME)
	if err := r.Client.Delete(context.TODO(), deployment); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("watchkeeper not found, deployment already deleted")
			return nil
		}
		reqLogger.Error(err, "could not delete deployment", "name", utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME)
		return err
	}
	return nil
}

// Begin installation or deletion of Catalog Source
func (r *RazeeDeploymentReconciler) createCatalogSource(instance *marketplacev1alpha1.RazeeDeployment, catalogName string) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "createCatalogSource", "Request.Namespace", instance.Namespace, "Request.Name", instance.Name)

	return reconcile.Result{}, retry.RetryOnConflict(retry.DefaultBackoff, func() error {

		catalogSrc := &operatorsv1alpha1.CatalogSource{}
		catalogSrcNamespacedName := types.NamespacedName{
			Name:      catalogName,
			Namespace: utils.OPERATOR_MKTPLACE_NS}

		// If InstallIBMCatalogSource is true: install Catalog Source
		// if InstallIBMCatalogSource is false: do not install Catalog Source, and delete existing one (if it exists)
		if ptr.ToBool(instance.Spec.InstallIBMCatalogSource) {
			// If the Catalog Source does not exist, create one
			err := r.Client.Get(context.TODO(), catalogSrcNamespacedName, catalogSrc)
			if err != nil && errors.IsNotFound(err) {
				// Create catalog source
				var newCatalogSrc *operatorsv1alpha1.CatalogSource
				if utils.IBM_CATALOGSRC_NAME == catalogName {
					newCatalogSrc = utils.BuildNewIBMCatalogSrc()
				} else { // utils.OPENCLOUD_CATALOGSRC_NAME
					newCatalogSrc = utils.BuildNewOpencloudCatalogSrc()
				}

				reqLogger.Info("Creating catalog source")
				if err := r.Client.Create(context.TODO(), newCatalogSrc); err != nil {
					reqLogger.Error(err, "Failed to create a CatalogSource.", "CatalogSource.Namespace ", newCatalogSrc.Namespace, "CatalogSource.Name", newCatalogSrc.Name)
					return err
				}

				ok := instance.Status.Conditions.SetCondition(status.Condition{
					Type:    marketplacev1alpha1.ConditionInstalling,
					Status:  corev1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonCatalogSourceInstall,
					Message: catalogName + " catalog source installed.",
				})

				if ok {
					reqLogger.Info("updating razeedeployment status")
					return r.Client.Status().Update(context.TODO(), instance)
				}

				return nil
			} else if err != nil {
				// Could not get catalog source
				reqLogger.Error(err, "Failed to get CatalogSource", "CatalogSource.Namespace ", catalogSrcNamespacedName.Namespace, "CatalogSource.Name", catalogSrcNamespacedName.Name)
				return err
			}
		} else {
			// Delete catalog source.
			catalogSrc.Name = catalogName
			catalogSrc.Namespace = utils.OPERATOR_MKTPLACE_NS
			if err := r.Client.Delete(context.TODO(), catalogSrc, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !errors.IsNotFound(err) {
				reqLogger.Info("Failed to delete the existing CatalogSource.", "CatalogSource.Namespace ", catalogSrc.Namespace, "CatalogSource.Name", catalogSrc.Name)
				return err
			}

			ok := instance.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonCatalogSourceDelete,
				Message: catalogName + " catalog source deleted.",
			})

			if ok {
				reqLogger.Info("updating razeedeployment status")
				return r.Client.Status().Update(context.TODO(), instance)
			}

		}

		return nil
	})
}

func isMapStringByteEqual(d1, d2 map[string][]byte) bool {
	for key, value := range d1 {
		value2, ok := d2[key]
		if !ok {
			return false
		}

		if !bytes.Equal(value, value2) {
			return false
		}
	}

	for key := range d2 {
		_, ok := d1[key]
		if !ok {
			return false
		}
	}

	return true
}
