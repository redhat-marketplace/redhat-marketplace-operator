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

	"golang.org/x/exp/slices"

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/operrors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"

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
	cfg                  *config.OperatorConfig
	factory              *manifests.Factory
	recorder             record.EventRecorder
	prometheusAPIBuilder *prometheus.PrometheusAPIBuilder
}

func (r *MeterBaseReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *MeterBaseReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
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

		return err == nil
	})

	mgr.Add(secretMapHandler)

	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MeterBase{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
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
			&source.Kind{Type: &marketplacev1beta1.MeterDefinition{}},
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
			return r.Client.Update(context.TODO(), instance)
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

	if instance.Status.Conditions == nil {
		instance.Status.Conditions = status.Conditions{}
	}

	userWorkloadMonitoringEnabledOnCluster, err := isUserWorkloadMonitoringEnabledOnCluster(r.Client, r.cfg.Infrastructure, reqLogger)
	if err != nil {
		reqLogger.Error(err, "failed to get user workload monitoring")
	}

	// With the removal of RHM Prometheus, UserWorkloadMonitoringEnabled should now always be true
	if !ptr.ToBool(instance.Spec.UserWorkloadMonitoringEnabled) {
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			instance.Spec.UserWorkloadMonitoringEnabled = ptr.Bool(true)
			return r.Client.Update(context.TODO(), instance)
		}); err != nil {
			return reconcile.Result{}, err
		}
	}
	userWorkloadMonitoringEnabledSpec := true

	userWorkloadConfigurationIsValid, userWorkloadErr := validateUserWorkLoadMonitoringConfig(r.Client, reqLogger)
	if userWorkloadErr != nil {
		reqLogger.Info(userWorkloadErr.Error())
	}

	// userWorkloadMonitoringEnabled is considered enabled if the Spec,cluster configuration,and user workload config validation are satisfied
	// userWorkloadMonitoringEnabled := userWorkloadMonitoringEnabledOnCluster && userWorkloadMonitoringEnabledSpec && userWorkloadConfigurationIsValid

	// set the condition of UserWorkloadMonitoring on Status
	userWorkloadMonitoringCondition := getUserWorkloadMonitoringCondition(userWorkloadMonitoringEnabledSpec,
		userWorkloadMonitoringEnabledOnCluster,
		userWorkloadConfigurationIsValid,
		userWorkloadErr)

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(userWorkloadMonitoringCondition) {
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// Start Install Condition
	if instance.Status.Conditions.IsUnknownFor(marketplacev1alpha1.ConditionInstalling) {
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			if instance.Status.Conditions.SetCondition(marketplacev1alpha1.MeterBaseStartInstall) {
				return r.Client.Status().Update(context.TODO(), instance)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	// ---
	// Install Objects
	// ---
	//

	// Uninstall old Prometheus
	if err := r.uninstallPrometheus(instance); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.checkUWMDefaultStorageClassPrereq(instance); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.installMetricStateDeployment(instance); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.installUserWorkloadMonitoring(instance); err != nil {
		return reconcile.Result{}, err
	}

	// Fetch the MarketplaceConfig instance for isDisconnected state
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
		result, err := r.createReporterCronJob(instance, userWorkloadMonitoringEnabledSpec, isDisconnected)
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

	// ----
	// Update Status
	// ----

	// Set status on meterbase reflecting user workload monitoring prometheus
	prometheusStatefulSet := &appsv1.StatefulSet{}
	promStsNamespacedName := types.NamespacedName{
		Namespace: utils.OPENSHIFT_USER_WORKLOAD_MONITORING_NAMESPACE,
		Name:      utils.OPENSHIFT_USER_WORKLOAD_MONITORING_STATEFULSET_NAME,
	}

	if err := r.Client.Get(context.TODO(), promStsNamespacedName, prometheusStatefulSet); kerrors.IsNotFound(err) {
		reqLogger.Info("can't find user workload monitoring prometheus statefulset, requeuing")
		return reconcile.Result{RequeueAfter: time.Second * 60}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}

		statusCopy := instance.Status.DeepCopy()

		instance.Status.Replicas = &prometheusStatefulSet.Status.Replicas
		instance.Status.UpdatedReplicas = &prometheusStatefulSet.Status.UpdatedReplicas
		instance.Status.AvailableReplicas = &prometheusStatefulSet.Status.ReadyReplicas
		instance.Status.UnavailableReplicas = ptr.Int32(
			prometheusStatefulSet.Status.CurrentReplicas - prometheusStatefulSet.Status.ReadyReplicas)

		if !reflect.DeepEqual(instance.Status, statusCopy) {
			return r.Client.Status().Update(context.TODO(), instance)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// Provide Status on Prometheus ActiveTargets
	targets, err := r.healthBadActiveTargets(userWorkloadMonitoringEnabledSpec, reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	var condition status.Condition
	if len(targets) == 0 {
		condition = marketplacev1alpha1.MeterBasePrometheusTargetGoodHealth
	} else {
		condition = marketplacev1alpha1.MeterBasePrometheusTargetBadHealth
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(condition) {
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// Finish Install Condition
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}
		if instance.Status.Conditions.SetCondition(marketplacev1alpha1.MeterBaseFinishInstall) {
			return r.Client.Status().Update(context.TODO(), instance)
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
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
	if meterdefNamespace == deployedNamespace && slices.Contains(meterdefSlice, meterdefName) {
		return true
	}
	return false
}

func (r *MeterBaseReconciler) installMetricStateDeployment(
	instance *marketplacev1alpha1.MeterBase,
) error {

	if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
		return r.factory.MetricStateDeployment()
	}); err != nil {
		return err
	}

	if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
		return r.factory.MetricStateService()
	}); err != nil {
		return err
	}

	if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
		return r.factory.MetricStateServiceMonitor(nil)
	}); err != nil {
		return err
	}

	if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
		return r.factory.MetricStateMeterDefinition()
	}); err != nil {
		return err
	}

	if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
		return r.factory.KubeStateMetricsService()
	}); err != nil {
		return err
	}

	if err := r.factory.CreateOrUpdate(r.Client, instance, func() (client.Object, error) {
		return r.factory.ReporterMeterDefinition()
	}); err != nil {
		return err
	}

	return nil
}

// Record a DefaultClassNotFound Event, but do not err
// User could possibly, but less likely, set up UWM storage without a default
func (r *MeterBaseReconciler) checkUWMDefaultStorageClassPrereq(instance *marketplacev1alpha1.MeterBase) error {
	_, err := utils.GetDefaultStorageClass(r.Client)
	if err != nil {
		if errors.Is(err, operrors.DefaultStorageClassNotFound) {
			r.recorder.Event(instance, "Warning", "DefaultClassNotFound", "Default storage class not found")
		} else {
			return err
		}
	}
	return nil
}

// Install the and MeterDefinition to monitor & report UserWorkloadMonitoring uptime
func (r *MeterBaseReconciler) installUserWorkloadMonitoring(instance *marketplacev1alpha1.MeterBase) error {

	if err := r.factory.CreateOrUpdate(r.Client, nil, func() (client.Object, error) {
		return r.factory.UserWorkloadMonitoringMeterDefinition()
	}); err != nil {
		return err
	}

	return nil
}

/*
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
*/

/*
func (r *MeterBaseReconciler) uninstallMetricState(
	instance *marketplacev1alpha1.MeterBase,
) []ClientAction {
	deployment, _ := r.factory.MetricStateDeployment()
	service, _ := r.factory.MetricStateService()
	sm0, _ := r.factory.MetricStateServiceMonitor(nil)
	sm1, _ := r.factory.KubeStateMetricsServiceMonitor()
	sm2, _ := r.factory.KubeletServiceMonitor()
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
*/

func (r *MeterBaseReconciler) uninstallPrometheus(instance *marketplacev1alpha1.MeterBase) error {
	secret0, _ := r.factory.PrometheusDatasources()
	secret1, _ := r.factory.PrometheusProxySecret()
	secret2, _ := r.factory.PrometheusHtpasswdSecret("foo")
	secret3, _ := r.factory.PrometheusRBACProxySecret()
	prom, _ := r.factory.NewPrometheusDeployment(instance, nil)
	service, _ := r.factory.PrometheusService(instance.Name)
	serviceMonitor, _ := r.factory.PrometheusServiceMonitor()
	meterDefinition, _ := r.factory.PrometheusMeterDefinition()
	operaterDeployment, _ := r.factory.NewPrometheusOperatorDeployment([]string{})
	operatorService, _ := r.factory.NewPrometheusOperatorService()

	if err := r.Client.Delete(context.TODO(), secret0); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	if err := r.Client.Delete(context.TODO(), secret1); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	if err := r.Client.Delete(context.TODO(), secret2); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	if err := r.Client.Delete(context.TODO(), secret3); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	if err := r.Client.Delete(context.TODO(), prom); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	if err := r.Client.Delete(context.TODO(), service); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	if err := r.Client.Delete(context.TODO(), serviceMonitor); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	if err := r.Client.Delete(context.TODO(), meterDefinition); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	if err := r.Client.Delete(context.TODO(), operaterDeployment); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	if err := r.Client.Delete(context.TODO(), operatorService); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	return nil
}

/*
func (r *MeterBaseReconciler) uninstallPrometheusServingCertsCABundle() error {
	configMap, err := r.factory.PrometheusServingCertsCABundle()
	if err != nil {
		return err
	}
	if err := r.Client.Delete(context.TODO(), configMap); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	return nil
}
*/

/*
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
*/

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

func validateUserWorkLoadMonitoringConfig(client client.Client, reqLogger logr.Logger) (bool, error) {
	reqLogger.Info("validating user-workload-monitoring-config configmap")

	uwmConfigMap := &corev1.ConfigMap{}
	cmNamespacedName := types.NamespacedName{
		Namespace: utils.OPENSHIFT_USER_WORKLOAD_MONITORING_NAMESPACE,
		Name:      utils.OPENSHIFT_USER_WORKLOAD_MONITORING_CONFIGMAP_NAME,
	}

	if err := client.Get(context.TODO(), cmNamespacedName, uwmConfigMap); kerrors.IsNotFound(err) {
		return false, ErrUserWorkloadMonitoringConfigNotFound
	} else if err != nil {
		reqLogger.Error(err, "Failed to get user-workload-monitoring-config configmap")
		return false, err
	}

	reqLogger.Info("found user-workload-monitoring-config configmap")
	return isUserWorkLoadMonitoringConfigValid(uwmConfigMap, reqLogger)
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
func isUserWorkloadMonitoringEnabledOnCluster(client client.Client, infrastructure *config.Infrastructure, reqLogger logr.Logger) (bool, error) {
	if !infrastructure.HasOpenshift() || infrastructure.HasOpenshift() && !infrastructure.OpenshiftParsedVersion().GTE(utils.ParsedVersion460) {
		reqLogger.Info("openshift is not 46 or this isn't an openshift cluster",
			"hasOpenshift", infrastructure.HasOpenshift(), "version", infrastructure.OpenshiftVersion())
		return false, nil
	}

	reqLogger.Info("attempting to get if userworkload monitoring is enabled on cluster")

	// Check if enableUserWorkload: true in cluster-monitoring-config

	clusterMonitorConfigMap := &corev1.ConfigMap{}
	cmNamespacedName := types.NamespacedName{
		Namespace: utils.OPENSHIFT_MONITORING_NAMESPACE,
		Name:      utils.OPENSHIFT_CLUSTER_MONITORING_CONFIGMAP_NAME,
	}

	if err := client.Get(context.TODO(), cmNamespacedName, clusterMonitorConfigMap); kerrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get cluster-monitoring-config configmap")
		return false, err
	}

	return isEnableUserWorkloadConfigMap(clusterMonitorConfigMap)
}

func getUserWorkloadMonitoringCondition(
	userWorkloadMonitoringEnabledSpec bool,
	userWorkloadMonitoringEnabledOnCluster bool,
	userWorkloadConfigurationSet bool,
	userWorkloadConfigurationErr error,
) status.Condition {
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
func (r *MeterBaseReconciler) healthBadActiveTargets(userWorkloadMonitoringEnabled bool, reqLogger logr.Logger) ([]common.Target, error) {
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

func containsString(slice []string, value string) bool {
	for _, sv := range slice {
		if sv == value {
			return true
		}
	}
	return false
}
