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
	"os"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	prom "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/operrors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"

	"emperror.dev/errors"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cmomanifests "github.com/openshift/cluster-monitoring-operator/pkg/manifests"

	"sigs.k8s.io/yaml"
)

const (
	DEFAULT_PROM_SERVER            = "prom/prometheus:v2.15.2"
	DEFAULT_CONFIGMAP_RELOAD       = "jimmidyson/configmap-reload:v0.3.0"
	RELATED_IMAGE_PROM_SERVER      = "RELATED_IMAGE_PROM_SERVER"
	RELATED_IMAGE_CONFIGMAP_RELOAD = "RELATED_IMAGE_CONFIGMAP_RELOAD"

	PROM_DEP_NEW_WARNING_MSG     = "Use of redhat-marketplace-operator Prometheus is deprecated. Configuration of user workload monitoring is required https://marketplace.redhat.com/en-us/documentation/red-hat-marketplace-operator#integration-with-openshift-container-platform-monitoring"
	PROM_DEP_UPGRADE_WARNING_MSG = "Use of redhat-marketplace-operator Prometheus is deprecated, and will be removed next release. Configure user workload monitoring https://marketplace.redhat.com/en-us/documentation/red-hat-marketplace-operator#integration-with-openshift-container-platform-monitoring"
)

var (
	ErrRetentionTime                        = errors.New("retention time must be at least 168h")
	ErrInsufficientStorageConfiguration     = errors.New("must allocate at least 40GiB of disk space")
	ErrParseUserWorkloadConfiguration       = errors.New("could not parse user workload configuration from user-workload-monitoring-config cm")
	ErrUserWorkloadMonitoringConfigNotFound = errors.New("user-workload-monitoring-config config map not found on cluster")
)

var (
	secretMapHandler *predicates.SyncedMapHandler
)

// blank assignment to verify that ReconcileMeterBase implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterBaseReconciler{}

// MeterBaseReconciler reconciles a MeterBase object
type MeterBaseReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client               client.Client
	Scheme               *runtime.Scheme
	Log                  logr.Logger
	CC                   ClientCommandRunner
	cfg                  *config.OperatorConfig
	factory              *manifests.Factory
	patcher              patch.Patcher
	recorder             record.EventRecorder
	prometheusAPIBuilder *prom.PrometheusAPIBuilder
}

func (r *MeterBaseReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *MeterBaseReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

func (r *MeterBaseReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.Log.Info("command runner")
	r.CC = ccp
	return nil
}

func (r *MeterBaseReconciler) InjectPatch(p patch.Patcher) error {
	r.patcher = p
	return nil
}

func (r *MeterBaseReconciler) InjectFactory(f *manifests.Factory) error {
	r.factory = f
	return nil
}

func (r *MeterBaseReconciler) InjectPrometheusAPIBuilder(b *prometheus.PrometheusAPIBuilder) error {
	r.prometheusAPIBuilder = b
	return nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *MeterBaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFn := func(a client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      "rhm-marketplaceconfig-meterbase",
					Namespace: a.GetNamespace(),
				},
			},
		}
	}

	namespacePredicate := predicates.NamespacePredicate(r.cfg.DeployedNamespace)
	r.recorder = mgr.GetEventRecorderFor("meterbase-controller")

	isOpenshiftMonitoringObj := func(name string, namespace string) bool {
		return (name == utils.OPENSHIFT_CLUSTER_MONITORING_CONFIGMAP_NAME && namespace == utils.OPENSHIFT_MONITORING_NAMESPACE) ||
			(name == utils.OPENSHIFT_USER_WORKLOAD_MONITORING_CONFIGMAP_NAME && namespace == utils.OPENSHIFT_USER_WORKLOAD_MONITORING_NAMESPACE) ||
			(name == utils.KUBELET_SERVING_CA_BUNDLE_NAME && namespace == utils.OPENSHIFT_MONITORING_NAMESPACE)
	}

	monitoringPred := predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isOpenshiftMonitoringObj(e.ObjectNew.GetName(), e.ObjectNew.GetNamespace())
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return isOpenshiftMonitoringObj(e.Object.GetName(), e.Object.GetNamespace())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isOpenshiftMonitoringObj(e.Object.GetName(), e.Object.GetNamespace())
		},
	}

	meterdefsPred := predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return reconcileForMeterDef(r.cfg.DeployedNamespace, e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return reconcileForMeterDef(r.cfg.DeployedNamespace, e.Object.GetNamespace(), e.Object.GetName())
		},
	}

	secretMapHandler = predicates.NewSyncedMapHandler(func(in types.NamespacedName) bool {
		secret := corev1.Secret{}
		err := mgr.GetClient().Get(context.Background(), in, &secret)

		if err != nil {
			return false
		}

		return true
	})

	mgr.Add(secretMapHandler)

	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MeterBase{}).
		Watches(
			&source.Kind{Type: &batchv1.CronJob{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &monitoringv1.Prometheus{}},
			&handler.EnqueueRequestForOwner{
				OwnerType: &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(monitoringPred)).
		Watches(
			&source.Kind{Type: &corev1.Service{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &monitoringv1.ServiceMonitor{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestForOwner{
				IsController: false,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &appsv1.StatefulSet{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &v1beta1.MeterDefinition{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(meterdefsPred)).
		Watches(
			&source.Kind{Type: &corev1.Namespace{}},
			handler.EnqueueRequestsFromMapFunc(mapFn)).Complete(r)
}

// +kubebuilder:rbac:groups="",resources=configmaps;namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=create
// +kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=update;patch;delete,resourceNames=serving-certs-ca-bundle;kubelet-serving-ca-bundle
// +kubebuilder:rbac:groups="",namespace=system,resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",namespace=system,resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",namespace=system,resources=services,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",namespace=system,resourceNames=rhm-prometheus-meterbase;prometheus-operator;rhm-metric-state-service;kube-state-metrics,resources=services,verbs=update;patch;delete
// +kubebuilder:rbac:groups="marketplace.redhat.com",namespace=system,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="storage.k8s.io",resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments,verbs=update;patch;delete,resourceNames=rhm-metric-state;prometheus-operator
// +kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=meterbases;meterbases/status;meterbases/finalizers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="monitoring.coreos.com",namespace=system,resources=prometheuses;servicemonitors,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="monitoring.coreos.com",namespace=system,resources=prometheuses,verbs=update;patch;delete,resourceNames=rhm-marketplaceconfig-meterbase
// +kubebuilder:rbac:groups="monitoring.coreos.com",namespace=system,resources=servicemonitors,verbs=update;patch;delete,resourceNames=rhm-metric-state;kube-state-metrics;rhm-prometheus-meterbase;redhat-marketplace-kubelet;prometheus-user-workload;redhat-marketplace-kube-state-metrics
// +kubebuilder:rbac:groups="operators.coreos.com",resources=subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=operatorgroups,verbs=get;list
// +kubebuilder:rbac:groups=batch;extensions,namespace=system,resources=cronjobs,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=batch;extensions,namespace=system,resources=cronjobs,verbs=update;patch;delete,resourceNames=rhm-meter-report-upload

// The operator SA token is used by rhm-prom ServiceMonitors. Must be able to scrape metrics.
// +kubebuilder:rbac:groups="",resources=nodes/metrics,verbs=get
// +kubebuilder:rbac:urls=/metrics,verbs=get

// Reconcile reads that state of the cluster for a MeterBase object and makes changes based on the state read
// and what is in the MeterBase.Spec
func (r *MeterBaseReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterBase")

	cc := r.CC

	// Fetch the MeterBase instance
	instance := &marketplacev1alpha1.MeterBase{}
	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if kerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Resource not found. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get MeterBase")
		return reconcile.Result{}, err
	}


	// Remove finalizer used by previous versions, ownerref gc deletion is used for cleanup
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if controllerutil.ContainsFinalizer(instance, utils.CONTROLLER_FINALIZER) {
			controllerutil.RemoveFinalizer(instance, utils.CONTROLLER_FINALIZER)
			if err := r.Client.Update(context.TODO(), instance); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// if instance.Enabled == false
	// return do nothing
	if !instance.Spec.Enabled {
		reqLogger.Info("MeterBase resource found but ignoring since metering is not enabled.")
		return reconcile.Result{}, nil
	}

	message := "Meter Base install starting"
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = status.Conditions{}
	}

	userWorkloadMonitoringEnabledOnCluster, err := isUserWorkloadMonitoringEnabledOnCluster(cc, r.cfg.Infrastructure, reqLogger)
	if err != nil {
		reqLogger.Error(err, "failed to get user workload monitoring")
	}

	var userWorkloadMonitoringEnabledSpec bool

	// if the value isn't specified, have userWorkloadMonitoringEnabledByDefault
	if instance.Spec.UserWorkloadMonitoringEnabled == nil {
		userWorkloadMonitoringEnabledSpec = true
	} else {
		userWorkloadMonitoringEnabledSpec = *instance.Spec.UserWorkloadMonitoringEnabled
	}

	userWorkloadConfigurationIsValid, userWorkloadErr := validateUserWorkLoadMonitoringConfig(cc, reqLogger)
	if userWorkloadErr != nil {
		reqLogger.Info(userWorkloadErr.Error())
	}

	// userWorkloadMonitoringEnabled is considered enabled if the Spec,cluster configuration,and user workload config validation are satisfied
	userWorkloadMonitoringEnabled := userWorkloadMonitoringEnabledOnCluster && userWorkloadMonitoringEnabledSpec && userWorkloadConfigurationIsValid

	// set the condition of UserWorkloadMonitoring on Status
	userWorkloadMonitoringCondition := getUserWorkloadMonitoringCondition(userWorkloadMonitoringEnabledSpec,
		userWorkloadMonitoringEnabledOnCluster,
		userWorkloadConfigurationIsValid,
		userWorkloadErr)

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(userWorkloadMonitoringCondition){
			if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}


	message = "Meter Base install started"
	if instance.Status.Conditions.IsUnknownFor(marketplacev1alpha1.ConditionInstalling) {
		if result, err := cc.Do(context.TODO(), UpdateStatusCondition(instance, &instance.Status.Conditions, status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonMeterBaseStartInstall,
			Message: message,
		})); result.Is(Error) || result.Is(Requeue) {
			if err != nil {
				return result.ReturnWithError(errors.Wrap(err, "error creating service monitor"))
			}

			return result.Return()
		}
	}

	// ---
	// Install Objects
	// ---
	//

	// Uninstall old Prom
	if result, _ := cc.Do(context.TODO(),
		Do(r.uninstallPrometheusOperator(instance)...),
		Do(r.uninstallPrometheus(instance)...),
	); !result.Is(Continue) {
		if result.Is(Error) {
			reqLogger.Error(result, "error in reconcile")
			return result.ReturnWithError(errors.Wrap(result, "error uninstalling prometheus"))
		}

		reqLogger.Info("returning result from prometheus uninstall", "result", *result)
		return result.Return()
	}
	

	// Openshift provides Prometheus
	if result, _ := cc.Do(context.TODO(),
		Do(r.checkUWMDefaultStorageClassPrereq(instance)...),
		Do(r.installPrometheusServingCertsCABundle()...),
		Do(r.installMetricStateDeployment(instance, userWorkloadMonitoringEnabled)...),
		Do(r.installUserWorkloadMonitoring(instance)...),
	); !result.Is(Continue) {
		if result.Is(Error) {
			reqLogger.Error(result, "error in reconcile")
			return result.ReturnWithError(errors.Wrap(result, "error creating metric-state"))
		}

		reqLogger.Info("returning result from openshift provides prometheus", "result", *result)
		return result.Return()
	}

	promStsNamespacedName := types.NamespacedName{
		Namespace: utils.OPENSHIFT_USER_WORKLOAD_MONITORING_NAMESPACE,
		Name:      utils.OPENSHIFT_USER_WORKLOAD_MONITORING_STATEFULSET_NAME,
	}
	
	// ----
	// Update our status
	// ----

	// Set status for prometheus
	prometheusStatefulset := &appsv1.StatefulSet{}
	if result, err := cc.Do(
		context.TODO(),
		GetAction(types.NamespacedName{
			Namespace: instance.Namespace,
			Name:      instance.Name,
		}, instance),
		HandleResult(
			GetAction(promStsNamespacedName, prometheusStatefulset),
			OnContinue(Call(func() (ClientAction, error) {
				updatedInstance := instance.DeepCopy()
				updatedInstance.Status.Replicas = &prometheusStatefulset.Status.Replicas
				updatedInstance.Status.UpdatedReplicas = &prometheusStatefulset.Status.UpdatedReplicas
				updatedInstance.Status.AvailableReplicas = &prometheusStatefulset.Status.ReadyReplicas
				updatedInstance.Status.UnavailableReplicas = ptr.Int32(
					prometheusStatefulset.Status.CurrentReplicas - prometheusStatefulset.Status.ReadyReplicas)

				var action ClientAction = nil

				reqLogger.Info("statefulset status", "status", updatedInstance.Status)

				if !reflect.DeepEqual(updatedInstance.Status, instance.Status) {
					reqLogger.Info("prometheus statefulset status is up to date")
					return HandleResult(UpdateAction(updatedInstance, UpdateStatusOnly(true)), OnContinue(action)), nil
				}

				return action, nil
			})),
			OnNotFound(Call(func() (ClientAction, error) {
				reqLogger.Info("can't find prometheus statefulset, requeuing")
				return RequeueAfterResponse(30 * time.Second), nil
			})),
		),
	); result.Is(Error) || result.Is(Requeue) {
		if err != nil {
			return result.ReturnWithError(errors.Wrap(err, "error updating status"))
		}

		return result.Return()
	}

	// Update final condition
	message = "Meter Base install complete"
	if result, err := cc.Do(context.TODO(), UpdateStatusCondition(instance, &instance.Status.Conditions, status.Condition{
		Type:    marketplacev1alpha1.ConditionInstalling,
		Status:  corev1.ConditionFalse,
		Reason:  marketplacev1alpha1.ReasonMeterBaseFinishInstall,
		Message: message,
	})); result.Is(Error) || result.Is(Requeue) {
		if err != nil {
			return result.ReturnWithError(errors.Wrap(err, "error creating service monitor"))
		}

		return result.Return()
	}



	// Fetch the MarketplaceConfig instance
	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.MARKETPLACECONFIG_NAME, Namespace: request.Namespace}, marketplaceConfig)
	if err != nil {
		reqLogger.Error(err, "Failed to get MarketplaceConfig instance")
		return reconcile.Result{}, err
	}

	isDisconnected := false
	if marketplaceConfig != nil && marketplaceConfig.Spec.IsDisconnected != nil && *marketplaceConfig.Spec.IsDisconnected {
		isDisconnected = true
	}

	// If DataService is enabled, create the CronJob that periodically uploads the Reports from the DataService
	if instance.Spec.IsDataServiceEnabled() {
		result, err := r.createReporterCronJob(instance, userWorkloadMonitoringEnabled, isDisconnected)
		if err != nil {
			reqLogger.Error(err, "Failed to createReporterCronJob")
			return result, err
		} else if result.Requeue || result.RequeueAfter != 0 {
			return result, err
		}
	} else {
		result, err := r.deleteReporterCronJob(isDisconnected)
		if err != nil {
			reqLogger.Error(err, "Failed to deleteReporterCronJob")
			return result, err
		} else if result.Requeue || result.RequeueAfter != 0 {
			return result, err
		}
	}

	// Provide Status on Prometheus ActiveTargets
	targets, err := r.healthBadActiveTargets(cc, userWorkloadMonitoringEnabled, reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	var condition status.Condition
	if len(targets) == 0 {
		condition = marketplacev1alpha1.MeterBasePrometheusTargetGoodHealth
	} else {
		condition = marketplacev1alpha1.MeterBasePrometheusTargetBadHealth
	}

	result, _ := cc.Do(context.TODO(), UpdateStatusCondition(instance, &instance.Status.Conditions, condition))
	if result.Is(Error) {
		reqLogger.Error(result.GetError(), "Failed to update status condition.")
		return result.Return()
	}

	result, _ = cc.Do(context.TODO(), UpdateAction(instance, UpdateStatusOnly(true)))
	if result.Is(Error) {
		reqLogger.Error(result.GetError(), "Failed to update status targets.")
		return result.Return()
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{RequeueAfter: time.Hour * 1}, nil
}

func getCategoriesFromMeterDefinitions(meterDefinitions []marketplacev1beta1.MeterDefinition) []string {
	var categoryList []string
	for _, meterDef := range meterDefinitions {
		v := meterDef.GetLabels()["marketplace.redhat.com/category"]
		if !containsString(categoryList, v) {
			categoryList = append(categoryList, v)
		}
	}
	return categoryList
}

func reconcileForMeterDef(deployedNamespace string, meterdefNamespace string, meterdefName string) bool {
	if meterdefNamespace == utils.OPENSHIFT_USER_WORKLOAD_MONITORING_NAMESPACE && meterdefName == utils.UserWorkloadMonitoringMeterdef {
		return true
	}

	var meterdefSlice = []string{utils.PrometheusMeterbaseUptimeMeterdef, utils.MetricStateUptimeMeterdef, utils.MeterReportJobFailedMeterdef}
	if meterdefNamespace == deployedNamespace && Contains(meterdefSlice, meterdefName) {
		return true
	}
	return false
}

const promServiceName = "rhm-prometheus-meterbase"

const tokenSecretName = "redhat-marketplace-service-account-token"

func (r *MeterBaseReconciler) createTokenSecret(instance *marketplacev1alpha1.MeterBase) []ClientAction {
	secretName := tokenSecretName
	secret := corev1.Secret{}

	return []ClientAction{
		HandleResult(
			GetAction(types.NamespacedName{
				Name:      secretName,
				Namespace: r.cfg.DeployedNamespace,
			}, &secret),
			OnNotFound(Call(func() (ClientAction, error) {
				secret.Name = secretName
				secret.Namespace = r.cfg.DeployedNamespace

				r.Log.Info("creating secret", "secret", secretName)

				tokenData, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
				if err != nil {
					return nil, err
				}

				secret.Data = map[string][]byte{
					"token": tokenData,
				}

				return CreateAction(&secret, CreateWithAddOwner(instance)), nil
			})),
			OnContinue(Call(func() (ClientAction, error) {
				tokenData, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
				if err != nil {
					return nil, err
				}

				r.Log.Info("found secret", "secret", secretName)

				if v, ok := secret.Data["token"]; !ok || ok && bytes.Compare(v, tokenData) != 0 {
					secret.Data = map[string][]byte{
						"token": tokenData,
					}
					r.Log.Info("updating secret", "secret", secretName)
					return UpdateAction(&secret), nil
				}
				return nil, nil
			})),
		),
	}
}

func (r *MeterBaseReconciler) installMetricStateDeployment(
	instance *marketplacev1alpha1.MeterBase,
	userWorkoadMonitoring bool,
) []ClientAction {
	pod := &corev1.Pod{}
	secret := &corev1.Secret{}
	secretList := &corev1.SecretList{}
	metricStateDeployment := &appsv1.Deployment{}
	metricStateService := &corev1.Service{}
	metricStateServiceMonitor := &monitoringv1.ServiceMonitor{}
	metricStateMeterDefinition := &marketplacev1beta1.MeterDefinition{}
	kubeStateMetricsService := &corev1.Service{}
	reporterMeterDefinition := &marketplacev1beta1.MeterDefinition{}

	args := manifests.CreateOrUpdateFactoryItemArgs{
		Owner:   instance,
		Patcher: r.patcher,
	}

	secretAction := Do(
		HandleResult(
			ListAction(
				secretList, client.InNamespace(r.cfg.DeployedNamespace), client.MatchingLabels{
					"name": "redhat-marketplace-service-account-token",
				}),
			OnContinue(Call(func() (ClientAction, error) {
				if secretList == nil || len(secretList.Items) == 0 {
					return manifests.CreateIfNotExistsFactoryItem(secret, func() (client.Object, error) {
						return r.factory.ServiceAccountPullSecret()
					}, CreateWithAddOwner(pod)), nil
				}

				if len(secretList.Items) > 1 {
					actions := []ClientAction{}

					for _, secret := range secretList.Items {
						actions = append(actions, DeleteAction(&secret))
					}

					return Do(actions...), nil
				}

				secret = &secretList.Items[0]

				secretMapHandler.AddOrUpdate(
					types.NamespacedName{
						Name:      secret.Name,
						Namespace: secret.Namespace,
					},
					types.NamespacedName{
						Name:      instance.Name,
						Namespace: instance.Namespace,
					},
				)
				return nil, nil
			}))),
		Call(func() (ClientAction, error) {
			if secret == nil {
				return nil, errors.New("secret -l name=redhat-marketplace-service-account-token secret not found")
			}

			secretMapHandler.AddOrUpdate(
				types.NamespacedName{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
				types.NamespacedName{
					Name:      instance.Name,
					Namespace: instance.Namespace,
				},
			)

			return nil, nil
		}),
	)

	actions := []ClientAction{
		HandleResult(
			GetAction(
				types.NamespacedName{Namespace: r.cfg.DeployedNamespace, Name: r.cfg.DeployedPodName},
				pod,
			),
			OnNotFound(ReturnWithError(errors.New("pod not found")))),
		secretAction,
		manifests.CreateOrUpdateFactoryItemAction(
			metricStateDeployment,
			func() (client.Object, error) {
				obj, err := r.factory.MetricStateDeployment()
				if err == nil {
					r.factory.SetControllerReference(instance, obj)
				}
				return obj, err
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			metricStateService,
			func() (client.Object, error) {
				obj, err := r.factory.MetricStateService()
				if err == nil {
					r.factory.SetControllerReference(instance, obj)
				}
				return obj, err
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			metricStateServiceMonitor,
			func() (client.Object, error) {
				obj, err := r.factory.MetricStateServiceMonitor(&secret.Name)
				if err == nil {
					r.factory.SetControllerReference(instance, obj)
				}
				return obj, err
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			metricStateMeterDefinition,
			func() (client.Object, error) {
				obj, err := r.factory.MetricStateMeterDefinition()
				if err == nil {
					r.factory.SetControllerReference(instance, obj)
				}
				return obj, err
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			kubeStateMetricsService,
			func() (client.Object, error) {
				obj, err := r.factory.KubeStateMetricsService()
				if err == nil {
					r.factory.SetControllerReference(instance, obj)
				}
				return obj, err
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			reporterMeterDefinition,
			func() (client.Object, error) {
				obj, err := r.factory.ReporterMeterDefinition()
				if err == nil {
					r.factory.SetControllerReference(instance, obj)
				}
				return obj, err
			},
			args,
		),
	}

	kubeStateMetricsServiceMonitor := &monitoringv1.ServiceMonitor{}
	kubeletServiceMonitor := &monitoringv1.ServiceMonitor{}

	if !userWorkoadMonitoring {
		actions = append(actions,
			secretAction,
			manifests.CreateOrUpdateFactoryItemAction(
				kubeStateMetricsServiceMonitor,
				func() (client.Object, error) {
					obj, err := r.factory.KubeStateMetricsServiceMonitor(&secret.Name)
					if err == nil {
						r.factory.SetControllerReference(instance, obj)
					}
					return obj, err
				},
				args,
			),
			manifests.CreateOrUpdateFactoryItemAction(
				kubeletServiceMonitor,
				func() (client.Object, error) {
					obj, err := r.factory.KubeletServiceMonitor(&secret.Name)
					if err == nil {
						r.factory.SetControllerReference(instance, obj)
					}
					return obj, err
				},
				args,
			),
		)
	} else {
		kubeStateMetricsServiceMonitor, _ = r.factory.KubeStateMetricsServiceMonitor(nil)
		kubeletServiceMonitor, _ = r.factory.KubeletServiceMonitor(nil)

		actions = append(actions,
			HandleResult(
				GetAction(types.NamespacedName{Namespace: kubeStateMetricsServiceMonitor.Namespace, Name: kubeStateMetricsServiceMonitor.Name}, kubeStateMetricsServiceMonitor),
				OnContinue(DeleteAction(kubeStateMetricsServiceMonitor))),
			HandleResult(
				GetAction(types.NamespacedName{Namespace: kubeletServiceMonitor.Namespace, Name: kubeletServiceMonitor.Name}, kubeletServiceMonitor),
				OnContinue(DeleteAction(kubeletServiceMonitor))),
		)
	}

	return actions
}

// Record a DefaultClassNotFound Event, but do not err
// User could possibly, but less likely, set up UWM storage without a default
func (r *MeterBaseReconciler) checkUWMDefaultStorageClassPrereq(
	instance *marketplacev1alpha1.MeterBase,
) []ClientAction {
	return []ClientAction{
		Call(func() (ClientAction, error) {
			_, err := utils.GetDefaultStorageClass(r.Client)
			if err != nil {
				if errors.Is(err, operrors.DefaultStorageClassNotFound) {
					r.recorder.Event(instance, "Warning", "DefaultClassNotFound", "Default storage class not found")
				} else {
					return nil, err
				}
			}
			return nil, nil
		})}
}

func (r *MeterBaseReconciler) installUserWorkloadMonitoring(
	instance *marketplacev1alpha1.MeterBase,
) []ClientAction {
	serviceMonitor := &monitoringv1.ServiceMonitor{}
	meterDefinition := &marketplacev1beta1.MeterDefinition{}

	args := manifests.CreateOrUpdateFactoryItemArgs{
		Owner:   instance,
		Patcher: r.patcher,
	}

	actions := []ClientAction{
		manifests.CreateOrUpdateFactoryItemAction(
			serviceMonitor,
			func() (client.Object, error) {
				return r.factory.UserWorkloadMonitoringServiceMonitor()
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			meterDefinition,
			func() (client.Object, error) {
				return r.factory.UserWorkloadMonitoringMeterDefinition()
			},
			args,
		),
	}
	return actions
}

func (r *MeterBaseReconciler) uninstallPrometheusOperator(
	instance *marketplacev1alpha1.MeterBase,
) []ClientAction {
	deployment, _ := r.factory.NewPrometheusOperatorDeployment([]string{})
	service, _ := r.factory.NewPrometheusOperatorService()

	return []ClientAction{
		HandleResult(
			GetAction(types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, service),
			OnContinue(DeleteAction(service))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: deployment.Namespace, Name: deployment.Name}, deployment),
			OnContinue(DeleteAction(deployment))),
	}
}

var ignoreKubeStateList = []string{
	"kube_configmap.*",
	"kube_cronjob.*",
	"kube_daemonset.*",
	"kube_deployment.*",
	"kube_endpoint.*",
	"kube_job.*",
	"kube_node_status.*",
	"kube_replicaset.*",
	"kube_secret.*",
	"kube_statefulset.*",
}




func (r *MeterBaseReconciler) uninstallMetricState(
	instance *marketplacev1alpha1.MeterBase,
) []ClientAction {
	deployment, _ := r.factory.MetricStateDeployment()
	service, _ := r.factory.MetricStateService()
	sm0, _ := r.factory.MetricStateServiceMonitor(nil)
	sm1, _ := r.factory.KubeStateMetricsServiceMonitor(nil)
	sm2, _ := r.factory.KubeletServiceMonitor(nil)
	sm3, _ := r.factory.KubeStateMetricsService()
	msmd, _ := r.factory.MetricStateMeterDefinition()
	rmd, _ := r.factory.ReporterMeterDefinition()

	return []ClientAction{
		HandleResult(
			GetAction(types.NamespacedName{Namespace: rmd.Namespace, Name: rmd.Name}, rmd),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(rmd))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: msmd.Namespace, Name: msmd.Name}, msmd),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(msmd))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: sm0.Namespace, Name: sm0.Name}, sm0),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(sm0))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: sm1.Namespace, Name: sm1.Name}, sm1),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(sm1))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: sm2.Namespace, Name: sm2.Name}, sm2),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(sm2))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: sm3.Namespace, Name: sm3.Name}, sm3),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(sm3))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, service),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(service))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: deployment.Namespace, Name: deployment.Name}, deployment),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(deployment))),
	}
}

func (r *MeterBaseReconciler) uninstallPrometheus(
	instance *marketplacev1alpha1.MeterBase,
) []ClientAction {
	secret0, _ := r.factory.PrometheusDatasources()
	secret1, _ := r.factory.PrometheusProxySecret()
	secret2, _ := r.factory.PrometheusHtpasswdSecret("foo")
	secret3, _ := r.factory.PrometheusRBACProxySecret()
	secrets := []*corev1.Secret{secret0, secret1, secret2, secret3}
	prom, _ := r.newPrometheusOperator(instance, nil)
	service, _ := r.factory.PrometheusService(instance.Name)
	serviceMonitor, _ := r.factory.PrometheusServiceMonitor()
	meterDefinition, _ := r.factory.PrometheusMeterDefinition()

	actions := []ClientAction{}
	for _, sec := range secrets {
		actions = append(actions,
			HandleResult(
				GetAction(
					types.NamespacedName{Namespace: sec.Namespace, Name: sec.Name}, sec),
				OnContinue(DeleteAction(sec))))
	}

	return append(actions,
		HandleResult(
			GetAction(types.NamespacedName{Namespace: meterDefinition.Namespace, Name: meterDefinition.Name}, meterDefinition),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(meterDefinition))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: serviceMonitor.Namespace, Name: serviceMonitor.Name}, serviceMonitor),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(serviceMonitor))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, service),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(service))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: prom.Namespace, Name: prom.Name}, prom),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(prom))),
	)
}

func (r *MeterBaseReconciler) installPrometheusServingCertsCABundle() []ClientAction {
	return []ClientAction{
		manifests.CreateIfNotExistsFactoryItem(
			&corev1.ConfigMap{},
			func() (client.Object, error) {
				return r.factory.PrometheusServingCertsCABundle()
			},
		),
	}
}

func (r *MeterBaseReconciler) uninstallPrometheusServingCertsCABundle() []ClientAction {
	cm0, _ := r.factory.PrometheusServingCertsCABundle()

	return []ClientAction{
		HandleResult(
			GetAction(
				types.NamespacedName{Namespace: cm0.Namespace, Name: cm0.Name}, cm0),
			OnContinue(DeleteAction(cm0))),
	}
}

func (r *MeterBaseReconciler) uninstallUserWorkloadMonitoring() []ClientAction {
	sm, _ := r.factory.UserWorkloadMonitoringServiceMonitor()
	md, _ := r.factory.UserWorkloadMonitoringMeterDefinition()

	return []ClientAction{
		HandleResult(
			GetAction(
				types.NamespacedName{Namespace: sm.Namespace, Name: sm.Name}, sm),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(sm))),
		HandleResult(
			GetAction(
				types.NamespacedName{Namespace: md.Namespace, Name: md.Name}, md),
			OnNotFound(ContinueResponse()),
			OnContinue(DeleteAction(md))),
	}
}




func (r *MeterBaseReconciler) newPrometheusOperator(
	cr *marketplacev1alpha1.MeterBase,
	cfg *corev1.Secret,
) (*monitoringv1.Prometheus, error) {
	//prom, err := r.factory.NewPrometheusDeployment(cr, cfg)

	prom :=monitoringv1.Prometheus{}
	prom.Name = utils.METERBASE_PROMETHEUS_OPERATOR_NAME
	prom.Namespace = cr.GetNamespace()
	
	return &prom, nil
}






func (r *MeterBaseReconciler) createReporterCronJob(instance *marketplacev1alpha1.MeterBase, userWorkloadEnabled bool, isDisconnected bool) (reconcile.Result, error) {
	cronJob, err := r.factory.NewReporterCronJob(userWorkloadEnabled, isDisconnected)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, cronJob, func() error {
			orig, err := r.factory.NewReporterCronJob(userWorkloadEnabled, isDisconnected)
			if err != nil {
				return err
			}
			r.factory.SetControllerReference(instance, cronJob)

			if !reflect.DeepEqual(cronJob.Spec.JobTemplate, orig.Spec.JobTemplate) {
				cronJob.Spec.JobTemplate = orig.Spec.JobTemplate
			}

			if cronJob.Spec.ConcurrencyPolicy != orig.Spec.ConcurrencyPolicy {
				cronJob.Spec.ConcurrencyPolicy = orig.Spec.ConcurrencyPolicy
			}

			if cronJob.Spec.FailedJobsHistoryLimit != orig.Spec.FailedJobsHistoryLimit {
				cronJob.Spec.FailedJobsHistoryLimit = orig.Spec.FailedJobsHistoryLimit
			}

			if cronJob.Spec.SuccessfulJobsHistoryLimit != orig.Spec.SuccessfulJobsHistoryLimit {
				cronJob.Spec.SuccessfulJobsHistoryLimit = orig.Spec.SuccessfulJobsHistoryLimit
			}

			var latestEnv corev1.EnvVar
			latestContainer := &cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
			for _, envVar := range latestContainer.Env {
				if envVar.Name == "IS_DISCONNECTED" {
					latestEnv = envVar
				}
			}

			var origEnv corev1.EnvVar
			origContainer := &orig.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
			for _, envVar := range origContainer.Env {
				if envVar.Name == "IS_DISCONNECTED" {
					latestEnv = envVar
				}
			}

			if latestEnv.Value != origEnv.Value {
				latestEnv.Value = origEnv.Value
			}

			return nil
		})
		return err
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func isUserWorkLoadMonitoringConfigValid(clusterMonitorConfigMap *corev1.ConfigMap, reqLogger logr.Logger) (bool, error) {
	config, ok := clusterMonitorConfigMap.Data["config.yaml"]
	if !ok {
		return false, ErrParseUserWorkloadConfiguration
	}

	uwmc := cmomanifests.UserWorkloadConfiguration{}
	err := yaml.Unmarshal([]byte(config), &uwmc)
	if err != nil {
		err = fmt.Errorf("%w: %s ", ErrParseUserWorkloadConfiguration, err.Error())
		return false, err
	}

	if uwmc.Prometheus == nil {
		err := fmt.Errorf("%w: %s ", ErrParseUserWorkloadConfiguration, "could not find prometheus spec in user workload config")
		return false, err
	}

	foundRetention, err := time.ParseDuration(uwmc.Prometheus.Retention)
	if err != nil {
		err = fmt.Errorf("%w: %s ", ErrParseUserWorkloadConfiguration, err.Error())
		return false, err
	}

	reqLogger.Info("found retention", "retention", foundRetention.Hours())

	wantedRetention, err := time.ParseDuration("168h")
	if err != nil {
		err = fmt.Errorf("%w: %s ", ErrParseUserWorkloadConfiguration, err.Error())
		return false, err
	}

	if float64(foundRetention) < float64(wantedRetention) {
		return false, ErrRetentionTime
	}

	if uwmc.Prometheus.VolumeClaimTemplate == nil {
		err := fmt.Errorf("%w: %s ", ErrParseUserWorkloadConfiguration, "could not find Prometheus.VolumeClaimTemplate in user workload config")
		return false, err
	}

	wantedStorage := resource.MustParse("40Gi")
	wantedStorageI64, _ := wantedStorage.AsInt64()
	foundStorageI64, _ := uwmc.Prometheus.VolumeClaimTemplate.Spec.Resources.Requests.Storage().AsInt64()

	reqLogger.Info("found storage", "storage", foundStorageI64)

	if foundStorageI64 < wantedStorageI64 {
		return false, ErrInsufficientStorageConfiguration
	}

	return true, nil
}

func validateUserWorkLoadMonitoringConfig(cc ClientCommandRunner, reqLogger logr.Logger) (bool, error) {
	reqLogger.Info("validating user-workload-monitoring-config cm")

	uwmc := &corev1.ConfigMap{}
	result, _ := cc.Do(
		context.TODO(),
		GetAction(types.NamespacedName{
			Namespace: utils.OPENSHIFT_USER_WORKLOAD_MONITORING_NAMESPACE,
			Name:      utils.OPENSHIFT_USER_WORKLOAD_MONITORING_CONFIGMAP_NAME,
		}, uwmc),
	)
	if result.Is(Error) {
		reqLogger.Error(result.GetError(), "Failed to get user-workload-monitoring-config.")
		return false, result
	} else if result.Is(NotFound) {
		return false, ErrUserWorkloadMonitoringConfigNotFound
	} else if result.Is(Continue) {
		reqLogger.Info("found user-workload-monitoring-config")
		enableUserWorkload, err := isUserWorkLoadMonitoringConfigValid(uwmc, reqLogger)
		if err != nil {
			return false, err
		}

		return enableUserWorkload, nil
	}

	return false, nil
}

func (r *MeterBaseReconciler) deleteReporterCronJob(isDisconnected bool) (reconcile.Result, error) {
	cronJob, err := r.factory.NewReporterCronJob(false, isDisconnected)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = r.Client.Delete(context.TODO(), cronJob)
	if err != nil && !kerrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func isEnableUserWorkloadConfigMap(clusterMonitorConfigMap *corev1.ConfigMap) (bool, error) {
	cmc := cmomanifests.ClusterMonitoringConfiguration{}
	config, ok := clusterMonitorConfigMap.Data["config.yaml"]
	if ok {
		err := yaml.Unmarshal([]byte(config), &cmc)
		if err != nil {
			return false, err
		}
		if cmc.UserWorkloadEnabled != nil {
			return *cmc.UserWorkloadEnabled, nil
		}
	}
	return false, nil
}

// If this is Openshift 4.6+ check if Monitoring for User Defined Projects is enabled
// https://docs.openshift.com/container-platform/4.6/monitoring/enabling-monitoring-for-user-defined-projects.html
func isUserWorkloadMonitoringEnabledOnCluster(cc ClientCommandRunner, infrastructure *config.Infrastructure, reqLogger logr.Logger) (bool, error) {
	if !infrastructure.HasOpenshift() || infrastructure.HasOpenshift() && !infrastructure.OpenshiftParsedVersion().GTE(utils.ParsedVersion460) {
		reqLogger.Info("openshift is not 46 or this isn't an openshift cluster",
			"hasOpenshift", infrastructure.HasOpenshift(), "version", infrastructure.OpenshiftVersion())
		return false, nil
	}

	reqLogger.Info("attempting to get if userworkload monitoring is enabled on cluster")

	// Check if enableUserWorkload: true in cluster-monitoring-config
	clusterMonitorConfigMap := &corev1.ConfigMap{}
	result, _ := cc.Do(
		context.TODO(),
		GetAction(types.NamespacedName{
			Namespace: utils.OPENSHIFT_MONITORING_NAMESPACE,
			Name:      utils.OPENSHIFT_CLUSTER_MONITORING_CONFIGMAP_NAME,
		}, clusterMonitorConfigMap),
	)
	if result.Is(Error) {
		reqLogger.Error(result.GetError(), "Failed to get cluster-monitoring-config.")
		return false, result
	} else if result.Is(NotFound) {
		return false, nil
	} else if result.Is(Continue) {
		enableUserWorkload, err := isEnableUserWorkloadConfigMap(clusterMonitorConfigMap)
		reqLogger.Info("found cluster-monitoring-config", "enabledUserWorkload", enableUserWorkload)
		if err != nil {
			reqLogger.Error(result.GetError(), "Failed to parse cluster-monitoring-config.")
			return false, err
		}
		return enableUserWorkload, nil
	}

	return false, nil
}

func getUserWorkloadMonitoringCondition(
	userWorkloadMonitoringEnabledSpec bool,
	userWorkloadMonitoringEnabledOnCluster bool,
	userWorkloadConfigurationSet bool,
	userWorkloadConfigurationErr error,
) (status.Condition) {
	var condition status.Condition
	
	if userWorkloadMonitoringEnabledSpec && userWorkloadMonitoringEnabledOnCluster && userWorkloadConfigurationSet {
		condition = marketplacev1alpha1.UserWorkloadMonitoringEnabled
	}

	if !userWorkloadMonitoringEnabledSpec {
		condition = marketplacev1alpha1.UserWorkloadMonitoringDisabledSpec
	}

	if !userWorkloadMonitoringEnabledOnCluster {
		condition = marketplacev1alpha1.UserWorkloadMonitoringDisabledOnCluster
	}

	if !userWorkloadConfigurationSet && userWorkloadConfigurationErr != nil {
		if errors.Is(userWorkloadConfigurationErr, ErrInsufficientStorageConfiguration) {
			condition := marketplacev1alpha1.UserWorkloadMonitoringStorageConfigurationErr
			condition.Message = userWorkloadConfigurationErr.Error()
		}

		if errors.Is(userWorkloadConfigurationErr, ErrRetentionTime) {
			condition := marketplacev1alpha1.UserWorkloadMonitoringRetentionTimeConfigurationErr
			condition.Message = userWorkloadConfigurationErr.Error()
		}

		if errors.Is(userWorkloadConfigurationErr, ErrParseUserWorkloadConfiguration) {
			condition := marketplacev1alpha1.UserWorkloadMonitoringParseUserWorkloadConfigurationErr
			condition.Message = userWorkloadConfigurationErr.Error()
		}

		if errors.Is(userWorkloadConfigurationErr, ErrUserWorkloadMonitoringConfigNotFound) {
			condition := marketplacev1alpha1.UserWorkloadMonitoringConfigNotFound
			condition.Message = userWorkloadConfigurationErr.Error()
		}
	}

	return condition
}

// Return Prometheus ActiveTargets with HealthBad or Unknown status
func (r *MeterBaseReconciler) healthBadActiveTargets(cc ClientCommandRunner, userWorkloadMonitoringEnabled bool, reqLogger logr.Logger) ([]common.Target, error) {
	targets := []common.Target{}

	/* Must use Prometheus and not Thanos Querier for userWorkloadMonitoring case
	   Thanos Querier does not provide Prometheus Targets()
	   Thus we only get the user workload targets */
	prometheusAPI, err := r.prometheusAPIBuilder.Get(r.prometheusAPIBuilder.GetAPITypeFromFlag(userWorkloadMonitoringEnabled))
	if err != nil {
		return []common.Target{}, err
	}

	reqLogger.Info("getting target discovery from prometheus")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	targetsResult, err := prometheusAPI.Targets(ctx)

	if err != nil {
		reqLogger.Error(err, "prometheus.Targets()")
		returnErr := errors.Wrap(err, "error with targets query")
		return targets, returnErr
	}

	for _, activeTarget := range targetsResult.Active {
		if activeTarget.Health != prometheusv1.HealthGood {
			targets = append(targets,
				common.Target{
					Labels:     activeTarget.Labels,
					ScrapeURL:  activeTarget.ScrapeURL,
					LastError:  activeTarget.LastError,
					LastScrape: activeTarget.LastScrape.String(),
					Health:     activeTarget.Health,
				},
			)
		}
	}

	return targets, nil
}

// getWatchNamespace returns the Namespace the operator should be watching for changes
func getWatchNamespace() (string, error) {
	// WatchNamespaceEnvVar is the constant for env variable WATCH_NAMESPACE
	// which specifies the Namespace to watch.
	// An empty value means the operator is running with cluster scope.
	var watchNamespaceEnvVar = "WATCH_NAMESPACE"

	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	return ns, nil
}

func containsString(slice []string, value string) bool {
	for _, sv := range slice {
		if sv == value {
			return true
		}
	}
	return false
}
