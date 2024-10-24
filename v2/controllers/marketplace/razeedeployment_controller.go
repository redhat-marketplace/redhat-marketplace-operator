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

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
	Cfg     *config.OperatorConfig
	Factory *manifests.Factory
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *RazeeDeploymentReconciler) SetupWithManager(mgr manager.Manager) error {

	// Only reconcile this deployment
	rdPred := predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == utils.RAZEE_NAME
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == utils.RAZEE_NAME
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetName() == utils.RAZEE_NAME
		},
	}

	// This mapFn will queue the default named razeedeployment
	mapFn := handler.MapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
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
		// WithEventFilter(predicates.NamespacePredicate(r.Cfg.DeployedNamespace)).
		For(&marketplacev1alpha1.RazeeDeployment{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}, rdPred)).
		Watches(&appsv1.Deployment{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &marketplacev1alpha1.RazeeDeployment{}, handler.OnlyControllerOwner())).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(p)).
		Watches(&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(pp)).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(cmp)).
		Watches(
			&marketplacev1alpha1.MarketplaceConfig{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &marketplacev1alpha1.MarketplaceConfig{}),
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace=system,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=update;patch;delete,resourceNames=watch-keeper-non-namespaced;watch-keeper-limit-poll;razee-cluster-metadata;watch-keeper-config
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=update;patch;delete,resourceNames=rhm-operator-secret;watch-keeper-secret;clustersubscription;rhm-cos-reader-key
// +kubebuilder:rbac:groups=apps,namespace=system,resources=deployments;deployments/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=razeedeployments;razeedeployments/finalizers;razeedeployments/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=marketplaceconfigs;marketplaceconfigs/finalizers;marketplaceconfigs/status,verbs=get;list;watch;create;update;patch;delete

// OwnerRef deletion chain via rhm-remoteresource-controller
// +kubebuilder:rbac:groups=deploy.razee.io,namespace=system,resources=remoteresources,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=deploy.razee.io,namespace=system,resources=remoteresources,verbs=update;patch;delete,resourceNames=child;parent

// operator_config
// +kubebuilder:rbac:groups="config.openshift.io",resources=clusterversions,verbs=get;list;watch

// Infrastructure Discovery
// +kubebuilder:rbac:groups="",namespace=system,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",namespace=system,resources=clusterserviceversions,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",namespace=system,resources=subscriptions,verbs=get;list;watch

// Reconcile reads that state of the cluster for a RazeeDeployment object and makes changes based on the state read
// and what is in the RazeeDeployment.Spec
func (r *RazeeDeploymentReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	// Do nothing if redhat-marketplace-deployment-operator is installed, which will reconcile RazeeDeployment
	// If only ibm-metrics-operator is installed, we will reconcile watch-keeper only
	deployment := &appsv1.Deployment{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.RHM_CONTROLLER_DEPLOYMENT_NAME,
		Namespace: request.Namespace,
	}, deployment); err != nil && !errors.IsNotFound(err) {
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	} else if err == nil {
		// deployment-operator found, do not reconcile
		return reconcile.Result{}, nil
	}

	reqLogger.Info("Reconciling RazeeDeployment")

	// Fetch the RazeeDeployment instance
	instance := &marketplacev1alpha1.RazeeDeployment{}
	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("RazeeDeployment resource not found. Ignoring since object must be deleted")
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

	// If this was an upgrade, and Deployment Operator was removed, delete remoteresource-controller
	if err := r.removeRazeeDeployments(instance); err != nil {
		return reconcile.Result{}, err
	}

	// if not enabled then exit
	if !instance.Spec.Enabled {
		reqLogger.Info("Razee not enabled")

		if err := r.removeWatchkeeperDeployment(instance); err != nil {
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

	reqLogger.V(0).Info("All required razee configuration values have been found")

	//
	// rhm-watch-keeper
	//
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
	if err := r.Factory.CreateOrUpdate(r.Client, nil, func() (client.Object, error) {
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
	if err := r.Factory.CreateOrUpdate(r.Client, nil, func() (client.Object, error) {
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
	if err := r.Factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
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
	if err := r.Factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
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
	if err := r.Factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
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
	if err := r.Factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
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

	if registrationEnabled {
		reqLogger.Info("registration enabled")

		if err := r.Factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
			dep, err := r.Factory.NewWatchKeeperDeployment()
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
	r.Factory.SetOwnerReference(instance, cm)
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
	r.Factory.SetOwnerReference(instance, cm)
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
	r.Factory.SetOwnerReference(instance, cm)
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
	r.Factory.SetOwnerReference(instance, cm)
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
	r.Factory.SetOwnerReference(instance, &secret)
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

	r.Factory.SetOwnerReference(instance, &secret)
	return secret, err
}

// Undeploy the razee deployment and parent
func (r *RazeeDeploymentReconciler) removeRazeeDeployments(
	req *marketplacev1alpha1.RazeeDeployment,
) error {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	// RemoteResource CR/CRD would be deleted with parent operator
	// Need to remove remoteresource-controller Deployment owned by RazeeDeployment CR

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RHM_REMOTE_RESOURCE_DEPLOYMENT_NAME,
			Namespace: req.Namespace,
		},
	}

	if err := r.Client.Delete(context.TODO(), deployment); err != nil {
		if errors.IsNotFound(err) { // already deleted
			return nil
		} else {
			return err
		}
	} else {
		reqLogger.Info("rr deployment deleted", "name", utils.RHM_REMOTE_RESOURCE_DEPLOYMENT_NAME)
	}

	return nil
}

// Undeploy the watchkeeper deployment
func (r *RazeeDeploymentReconciler) removeWatchkeeperDeployment(req *marketplacev1alpha1.RazeeDeployment) error {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Starting delete of watchkeeper deployment")

	// TargetNamespace may not be set on RazeeDeployment during cleanup
	var namespace string
	if len(ptr.ToString(req.Spec.TargetNamespace)) != 0 {
		namespace = ptr.ToString(req.Spec.TargetNamespace)
	} else {
		namespace = req.Namespace
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME,
			Namespace: namespace,
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
