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

package meterbase

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"

	merrors "emperror.dev/errors"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	status "github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	prom "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/spf13/pflag"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	DEFAULT_PROM_SERVER            = "prom/prometheus:v2.15.2"
	DEFAULT_CONFIGMAP_RELOAD       = "jimmidyson/configmap-reload:v0.3.0"
	RELATED_IMAGE_PROM_SERVER      = "RELATED_IMAGE_PROM_SERVER"
	RELATED_IMAGE_CONFIGMAP_RELOAD = "RELATED_IMAGE_CONFIGMAP_RELOAD"
)

//ConfigmapReload: "jimmidyson/configmap-reload:v0.3.0",
//Server:          "prom/prometheus:v2.15.2",

var (
	log = logf.Log.WithName("controller_meterbase")

	meterbaseFlagSet *pflag.FlagSet
)

func init() {
	meterbaseFlagSet = pflag.NewFlagSet("meterbase", pflag.ExitOnError)
}

func FlagSet() *pflag.FlagSet {
	return meterbaseFlagSet
}

// Add creates a new MeterBase Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
) error {
	reconciler := newReconciler(mgr, ccprovider)
	return add(mgr, reconciler)
}

// blank assignment to verify that ReconcileMeterBase implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMeterBase{}

// ReconcileMeterBase reconciles a MeterBase object
type ReconcileMeterBase struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	opts       *MeterbaseOpts
	ccprovider ClientCommandRunnerProvider
	patcher    patch.Patcher
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
) reconcile.Reconciler {
	promOpts := &MeterbaseOpts{
		PullPolicy: "IfNotPresent",
	}
	return &ReconcileMeterBase{
		client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		ccprovider: ccprovider,
		patcher:    patch.RHMDefaultPatcher,
		opts:       promOpts,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("meterbase-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MeterBase
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MeterBase{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// watch configmap
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	// watch prometheus
	err = c.Watch(&source.Kind{Type: &monitoringv1.Prometheus{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	// watch headless service
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &monitoringv1.ServiceMonitor{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	mapFn := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name:      "rhm-marketplaceconfig-meterbase",
					Namespace: "openshift-redhat-marketplace",
				}},
			}
		})

	err = c.Watch(
		&source.Kind{Type: &corev1.Namespace{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: mapFn,
		})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a MeterBase object and makes changes based on the state read
// and what is in the MeterBase.Spec
func (r *ReconcileMeterBase) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterBase")

	cc := r.ccprovider.NewCommandRunner(r.client, r.scheme, reqLogger)

	// Fetch the MeterBase instance
	instance := &marketplacev1alpha1.MeterBase{}
	result, _ := cc.Do(
		context.TODO(),
		HandleResult(
			GetAction(
				request.NamespacedName,
				instance,
			),
		),
	)

	if !result.Is(Continue) {
		if result.Is(NotFound) {
			reqLogger.Info("MeterBase resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}

		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get MeterBase.")
		}

		return result.Return()
	}

	c := manifests.NewDefaultConfig()
	factory := manifests.NewFactory(instance.Namespace, c)

	// Execute the finalizer, will only run if we are in delete state
	if result, _ = cc.Do(
		context.TODO(),
		Call(SetFinalizer(instance, utils.CONTROLLER_FINALIZER)),
		Call(
			RunFinalizer(instance, utils.CONTROLLER_FINALIZER,
				Do(r.uninstallPrometheusOperator(instance, factory)...),
				Do(r.uninstallPrometheus(instance, factory)...),
				Do(r.uninstallMetricState(instance, factory)...),
			)),
	); !result.Is(Continue) {

		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get MeterBase.")
		}

		if result.Is(Return) {
			reqLogger.Info("Delete is complete.")
		}

		return result.Return()
	}

	// if instance.Enabled == false
	// return do nothing
	if !instance.Spec.Enabled {
		reqLogger.Info("MeterBase resource found but ignoring since metering is not enabled.")
		return reconcile.Result{}, nil
	}

	message := "Meter Base install starting"
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = &status.Conditions{}
	}

	message = "Meter Base install started"
	if instance.Status.Conditions.IsUnknownFor(marketplacev1alpha1.ConditionInstalling) {
		if result, err := cc.Do(context.TODO(), UpdateStatusCondition(instance, instance.Status.Conditions, status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonMeterBaseStartInstall,
			Message: message,
		})); result.Is(Error) || result.Is(Requeue) {
			if err != nil {
				return result.ReturnWithError(merrors.Wrap(err, "error creating service monitor"))
			}

			return result.Return()
		}
	}

	// ---
	// Install Objects
	// ---

	cfg := &corev1.Secret{}
	prometheus := &monitoringv1.Prometheus{}
	if result, _ := cc.Do(context.TODO(),
		Do(r.reconcilePrometheusOperator(instance, factory)...),
		Do(r.installMetricStateDeployment(instance, factory)...),
		Do(r.reconcileAdditionalConfigSecret(cc, instance, prometheus, factory, cfg)...),
		Do(r.reconcilePrometheus(instance, prometheus, factory, cfg)...),
	); !result.Is(Continue) {
		if result.Is(Error) {
			reqLogger.Error(result, "error in reconcile")
			return result.ReturnWithError(merrors.Wrap(result, "error creating prometheus"))
		}

		reqLogger.Info("returing result", "result", *result)
		return result.Return()
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
			GetAction(types.NamespacedName{
				Namespace: prometheus.Namespace,
				Name:      fmt.Sprintf("prometheus-%s", prometheus.Name),
			}, prometheusStatefulset),
			OnContinue(Call(func() (ClientAction, error) {
				updatedInstance := instance.DeepCopy()
				updatedInstance.Status.Replicas = &prometheusStatefulset.Status.Replicas
				updatedInstance.Status.UpdatedReplicas = &prometheusStatefulset.Status.UpdatedReplicas
				updatedInstance.Status.AvailableReplicas = &prometheusStatefulset.Status.ReadyReplicas
				updatedInstance.Status.UnavailableReplicas = ptr.Int32(
					prometheusStatefulset.Status.CurrentReplicas - prometheusStatefulset.Status.ReadyReplicas)

				var action ClientAction = nil

				reqLogger.Info("statefulset status", "status", updatedInstance.Status)

				if prometheusStatefulset.Status.Replicas != prometheusStatefulset.Status.ReadyReplicas {
					reqLogger.Info("prometheus statefulset has not finished roll out",
						"replicas", prometheusStatefulset.Status.Replicas,
						"ready", prometheusStatefulset.Status.ReadyReplicas)
					action = RequeueAfterResponse(5 * time.Second)
				}

				if !reflect.DeepEqual(updatedInstance.Status, instance.Status) {
					reqLogger.Info("prometheus statefulset status is up to date")
					return HandleResult(UpdateAction(updatedInstance, UpdateStatusOnly(true)), OnContinue(action)), nil
				}

				return action, nil
			})),
			OnNotFound(Call(func() (ClientAction, error) {
				log.Info("can't find prometheus statefulset, requeuing")
				return RequeueAfterResponse(30 * time.Second), nil
			})),
		),
	); result.Is(Error) || result.Is(Requeue) {
		if err != nil {
			return result.ReturnWithError(merrors.Wrap(err, "error creating service monitor"))
		}

		return result.Return()
	}

	// Update final condition

	message = "Meter Base install complete"
	if result, err := cc.Do(context.TODO(), UpdateStatusCondition(instance, instance.Status.Conditions, status.Condition{
		Type:    marketplacev1alpha1.ConditionInstalling,
		Status:  corev1.ConditionFalse,
		Reason:  marketplacev1alpha1.ReasonMeterBaseFinishInstall,
		Message: message,
	})); result.Is(Error) || result.Is(Requeue) {
		if err != nil {
			return result.ReturnWithError(merrors.Wrap(err, "error creating service monitor"))
		}

		return result.Return()
	}

	meterReportList := &marketplacev1alpha1.MeterReportList{}
	if result, err := cc.Do(
		context.TODO(),
		HandleResult(
			ListAction(meterReportList, client.InNamespace(request.Namespace)),
			OnContinue(Call(func() (ClientAction, error) {
				loc := time.UTC
				dateRangeInDays := -30

				meterReportNames := r.sortMeterReports(meterReportList)

				// prune old reports
				meterReportNames, err := r.removeOldReports(meterReportNames, loc, dateRangeInDays, request)
				if err != nil {
					reqLogger.Error(err, err.Error())
				}

				// fill in gaps of missing reports
				// we want the min date to be install date - 1 day
				endDate := time.Now().In(loc)

				minDate := instance.ObjectMeta.CreationTimestamp.Time.In(loc)
				minDate = utils.TruncateTime(minDate, loc)

				expectedCreatedDates := r.generateExpectedDates(endDate, loc, dateRangeInDays, minDate)
				foundCreatedDates, err := r.generateFoundCreatedDates(meterReportNames)

				if err != nil {
					return nil, err
				}

				log.Info("report dates", "expected", expectedCreatedDates, "found", foundCreatedDates, "min", minDate)
				err = r.createReportIfNotFound(expectedCreatedDates, foundCreatedDates, request, instance)

				if err != nil {
					return nil, err
				}

				return nil, nil
			})),
			OnNotFound(Call(func() (ClientAction, error) {
				log.Info("can't find meter report list, requeuing")
				return RequeueAfterResponse(30 * time.Second), nil
			})),
		),
	); result.Is(Error) || result.Is(Requeue) {
		if err != nil {
			return result.ReturnWithError(merrors.Wrap(err, "error creating service monitor"))
		}

		return result.Return()
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{RequeueAfter: time.Hour * 1}, nil
}

const promServiceName = "rhm-prometheus-meterbase"

func (r *ReconcileMeterBase) createReportIfNotFound(expectedCreatedDates []string, foundCreatedDates []string, request reconcile.Request, instance *marketplacev1alpha1.MeterBase) error {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	// find the diff between the dates we expect and the dates found on the cluster and create any missing reports
	missingReports := utils.FindDiff(expectedCreatedDates, foundCreatedDates)
	for _, missingReportDateString := range missingReports {
		missingReportName := r.newMeterReportNameFromString(missingReportDateString)
		missingReportStartDate, _ := time.Parse(utils.DATE_FORMAT, missingReportDateString)
		missingReportEndDate := missingReportStartDate.AddDate(0, 0, 1)

		missingMeterReport := r.newMeterReport(request.Namespace, missingReportStartDate, missingReportEndDate, missingReportName, instance, promServiceName)
		err := r.client.Create(context.TODO(), missingMeterReport)
		if err != nil {
			return err
		}
		reqLogger.Info("Created Missing Report", "Resource", missingReportName)
	}

	return nil
}

func (r *ReconcileMeterBase) removeOldReports(meterReportNames []string, loc *time.Location, dateRange int, request reconcile.Request) ([]string, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	limit := utils.TruncateTime(time.Now(), loc).AddDate(0, 0, dateRange)
	for _, reportName := range meterReportNames {
		dateCreated, err := r.retrieveCreatedDate(reportName)

		if err != nil {
			continue
		}

		if dateCreated.Before(limit) {
			reqLogger.Info("Deleting Report", "Resource", reportName)
			meterReportNames = utils.RemoveKey(meterReportNames, reportName)
			deleteReport := &marketplacev1alpha1.MeterReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      reportName,
					Namespace: request.Namespace,
				},
			}
			err := r.client.Delete(context.TODO(), deleteReport)
			if err != nil {
				return nil, err
			}
		}
	}

	return meterReportNames, nil
}

// configPath: /etc/config/prometheus.yml

type Images struct {
	ConfigmapReload string
	Server          string
}

type MeterbaseOpts struct {
	corev1.PullPolicy
}

func (r *ReconcileMeterBase) sortMeterReports(meterReportList *marketplacev1alpha1.MeterReportList) []string {

	var meterReportNames []string
	for _, report := range meterReportList.Items {
		meterReportNames = append(meterReportNames, report.Name)
	}

	sort.Strings(meterReportNames)
	return meterReportNames
}

func (r *ReconcileMeterBase) retrieveCreatedDate(reportName string) (time.Time, error) {
	splitStr := strings.SplitN(reportName, "-", 3)

	if len(splitStr) != 3 {
		return time.Now(), errors.New("failed to get date")
	}

	dateString := splitStr[2:]
	return time.Parse(utils.DATE_FORMAT, strings.Join(dateString, ""))
}

func (r *ReconcileMeterBase) newMeterReportNameFromDate(date time.Time) string {
	dateSuffix := strings.Join(strings.Fields(date.String())[:1], "")
	return fmt.Sprintf("%s%s", utils.METER_REPORT_PREFIX, dateSuffix)
}

func (r *ReconcileMeterBase) newMeterReportNameFromString(dateString string) string {
	dateSuffix := dateString
	return fmt.Sprintf("%s%s", utils.METER_REPORT_PREFIX, dateSuffix)
}

func (r *ReconcileMeterBase) generateFoundCreatedDates(meterReportNames []string) ([]string, error) {
	var foundCreatedDates []string
	for _, reportName := range meterReportNames {
		splitStr := strings.SplitN(reportName, "-", 3)

		if len(splitStr) != 3 {
			log.Info("meterreport name was irregular", "name", reportName)
			continue
		}

		dateString := splitStr[2:]
		foundCreatedDates = append(foundCreatedDates, strings.Join(dateString, ""))
	}
	return foundCreatedDates, nil
}

func (r *ReconcileMeterBase) generateExpectedDates(endTime time.Time, loc *time.Location, dateRange int, minDate time.Time) []string {
	// set start date
	startDate := utils.TruncateTime(endTime, loc).AddDate(0, 0, dateRange)

	if minDate.After(startDate) {
		startDate = utils.TruncateTime(minDate, loc)
	}

	// set end date
	endDate := utils.TruncateTime(endTime, loc)

	// loop through the range of dates we expect
	var expectedCreatedDates []string
	for d := startDate; d.After(endDate) == false; d = d.AddDate(0, 0, 1) {
		expectedCreatedDates = append(expectedCreatedDates, d.Format(utils.DATE_FORMAT))
	}

	return expectedCreatedDates
}

func (r *ReconcileMeterBase) newMeterReport(namespace string, startTime time.Time, endTime time.Time, meterReportName string, instance *marketplacev1alpha1.MeterBase, prometheusServiceName string) *marketplacev1alpha1.MeterReport {
	return &marketplacev1alpha1.MeterReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meterReportName,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MeterReportSpec{
			StartTime: metav1.NewTime(startTime),
			EndTime:   metav1.NewTime(endTime),
			PrometheusService: &common.ServiceReference{
				Name:       prometheusServiceName,
				Namespace:  instance.Namespace,
				TargetPort: intstr.FromString("rbac"),
			},
		},
	}
}

func (r *ReconcileMeterBase) reconcilePrometheusSubscription(
	instance *marketplacev1alpha1.MeterBase,
	subscription *olmv1alpha1.Subscription,
) []ClientAction {
	newSub := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                "beta",
			InstallPlanApproval:    olmv1alpha1.ApprovalAutomatic,
			Package:                "prometheus",
			CatalogSource:          "community-operators",
			CatalogSourceNamespace: "openshift-marketplace",
		},
	}

	return []ClientAction{
		HandleResult(
			GetAction(
				types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
				subscription,
			), OnNotFound(
				CreateAction(
					newSub,
					CreateWithAddOwner(instance),
				),
			)),
	}
}

func (r *ReconcileMeterBase) reconcilePrometheusOperator(
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
) []ClientAction {
	reqLogger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	nsList := &corev1.NamespaceList{}
	cm := &corev1.ConfigMap{}
	deployment := &appsv1.Deployment{}
	service := &corev1.Service{}

	args := manifests.CreateOrUpdateFactoryItemArgs{
		Owner:   instance,
		Patcher: r.patcher,
	}

	nsLabelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "openshift.io/cluster-monitoring",
				Operator: "DoesNotExist",
			},
		},
	})

	return []ClientAction{
		ListAction(nsList, client.MatchingLabelsSelector{
			Selector: nsLabelSelector,
		}),
		manifests.CreateIfNotExistsFactoryItem(
			cm,
			func() (runtime.Object, error) {
				return factory.NewPrometheusOperatorCertsCABundle()
			},
		),
		manifests.CreateOrUpdateFactoryItemAction(
			service,
			func() (runtime.Object, error) {
				return factory.NewPrometheusOperatorService()
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			deployment,
			func() (runtime.Object, error) {
				nsValues := []string{instance.Namespace}
				for _, ns := range nsList.Items {
					nsValues = append(nsValues, ns.Name)
				}
				sort.Strings(nsValues)
				reqLogger.Info("found namespaces", "ns", nsValues)
				return factory.NewPrometheusOperatorDeployment(nsValues)
			},
			args,
		),
	}
}

func (r *ReconcileMeterBase) installMetricStateDeployment(
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
) []ClientAction {
	deployment := &appsv1.Deployment{}
	service := &corev1.Service{}
	serviceMonitor := &monitoringv1.ServiceMonitor{}

	args := manifests.CreateOrUpdateFactoryItemArgs{
		Owner:   instance,
		Patcher: r.patcher,
	}

	return []ClientAction{
		manifests.CreateOrUpdateFactoryItemAction(
			deployment,
			func() (runtime.Object, error) {
				return factory.MetricStateDeployment()
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			service,
			func() (runtime.Object, error) {
				return factory.MetricStateService()
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			serviceMonitor,
			func() (runtime.Object, error) {
				return factory.MetricStateServiceMonitor()
			},
			args,
		),
	}
}

func (r *ReconcileMeterBase) uninstallPrometheusOperator(
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
) []ClientAction {
	cm, _ := factory.NewPrometheusOperatorCertsCABundle()
	deployment, _ := factory.NewPrometheusOperatorDeployment([]string{})
	service, _ := factory.NewPrometheusOperatorService()

	return []ClientAction{
		HandleResult(
			GetAction(types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, service),
			OnContinue(DeleteAction(service))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: deployment.Namespace, Name: deployment.Name}, deployment),
			OnContinue(DeleteAction(deployment))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: cm.Namespace, Name: cm.Name}, cm),
			OnContinue(DeleteAction(cm))),
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

func (r *ReconcileMeterBase) reconcileAdditionalConfigSecret(
	cc ClientCommandRunner,
	instance *marketplacev1alpha1.MeterBase,
	prometheus *monitoringv1.Prometheus,
	factory *manifests.Factory,
	additionalConfigSecret *corev1.Secret,
) []ClientAction {
	openshiftKubeletMonitor := &monitoringv1.ServiceMonitor{}
	openshiftKubeStateMonitor := &monitoringv1.ServiceMonitor{}
	metricStateMonitor := &monitoringv1.ServiceMonitor{}
	secretsInNamespace := &corev1.SecretList{}

	sm, err := factory.MetricStateServiceMonitor()

	if err != nil {
		log.Error(err, "error getting metric state")
	}

	return []ClientAction{
		Do(
			HandleResult(
				Do(
					GetAction(types.NamespacedName{
						Namespace: "openshift-monitoring",
						Name:      "kubelet",
					}, openshiftKubeletMonitor),
					GetAction(types.NamespacedName{
						Namespace: "openshift-monitoring",
						Name:      "kube-state-metrics",
					}, openshiftKubeStateMonitor),
					GetAction(types.NamespacedName{
						Namespace: sm.ObjectMeta.Namespace,
						Name:      sm.ObjectMeta.Name,
					}, metricStateMonitor),
					ListAction(secretsInNamespace, client.InNamespace(prometheus.GetNamespace()))),
				OnNotFound(ReturnWithError(errors.New("required serviceMonitor not found"))),
				OnError(ReturnWithError(errors.New("required serviceMonitor errored")))),
		),
		Call(func() (ClientAction, error) {
			newEndpoints := []monitoringv1.Endpoint{}

			for _, ep := range openshiftKubeStateMonitor.Spec.Endpoints {
				newEp := ep.DeepCopy()
				configs := []*monitoringv1.RelabelConfig{
					{
						SourceLabels: []string{"__name__"},
						Action:       "drop",
						Regex:        fmt.Sprintf("(%s)", strings.Join(ignoreKubeStateList, "|")),
					},
				}
				newEp.RelabelConfigs = append(configs, ep.RelabelConfigs...)
				newEndpoints = append(newEndpoints, *newEp)
			}

			openshiftKubeStateMonitor.Spec.Endpoints = newEndpoints

			sMons := map[string]*monitoringv1.ServiceMonitor{
				"kube-state": openshiftKubeStateMonitor,
				"kubelet":    openshiftKubeletMonitor,
			}
			sMons[metricStateMonitor.Name] = metricStateMonitor

			cfgGen := prom.NewConfigGenerator(log)

			basicAuthSecrets, err := loadBasicAuthSecrets(r.client, sMons, prometheus.Spec.RemoteRead, prometheus.Spec.RemoteWrite, prometheus.Spec.APIServerConfig, secretsInNamespace)
			if err != nil {
				return nil, err
			}

			bearerTokens, err := loadBearerTokensFromSecrets(r.client, sMons)
			if err != nil {
				return nil, err
			}

			cfg, err := cfgGen.GenerateConfig(prometheus, sMons, basicAuthSecrets, bearerTokens, []string{})

			if err != nil {
				return nil, err
			}

			sec, err := factory.PrometheusAdditionalConfigSecret(cfg)

			if err != nil {
				return nil, err
			}

			key, err := client.ObjectKeyFromObject(sec)

			if err != nil {
				return nil, err
			}

			return HandleResult(
				GetAction(key, additionalConfigSecret),
				OnNotFound(CreateAction(sec, CreateWithAddOwner(instance))),
				OnContinue(Call(func() (ClientAction, error) {

					if reflect.DeepEqual(additionalConfigSecret.Data, sec.Data) {
						return nil, nil
					}

					return UpdateAction(sec), nil
				}))), nil
		}),
	}
}

func (r *ReconcileMeterBase) reconcilePrometheus(
	instance *marketplacev1alpha1.MeterBase,
	prometheus *monitoringv1.Prometheus,
	factory *manifests.Factory,
	configSecret *corev1.Secret,
) []ClientAction {
	args := manifests.CreateOrUpdateFactoryItemArgs{
		Owner:   instance,
		Patcher: r.patcher,
	}

	dataSecret := &corev1.Secret{}
	kubeletCertsCM := &corev1.ConfigMap{}

	return []ClientAction{
		manifests.CreateIfNotExistsFactoryItem(
			&corev1.ConfigMap{},
			func() (runtime.Object, error) {
				return factory.PrometheusServingCertsCABundle()
			},
		),
		manifests.CreateIfNotExistsFactoryItem(
			dataSecret,
			func() (runtime.Object, error) {
				return factory.PrometheusDatasources()
			}),
		manifests.CreateIfNotExistsFactoryItem(
			&corev1.Secret{},
			func() (runtime.Object, error) {
				return factory.PrometheusProxySecret()
			}),
		manifests.CreateIfNotExistsFactoryItem(
			&corev1.Secret{},
			func() (runtime.Object, error) {
				return factory.PrometheusRBACProxySecret()
			},
		),
		HandleResult(manifests.CreateIfNotExistsFactoryItem(
			&corev1.Secret{},
			func() (runtime.Object, error) {
				data, ok := dataSecret.Data["basicAuthSecret"]

				if !ok {
					return nil, merrors.New("basicAuthSecret not on data")
				}

				return factory.PrometheusHtpasswdSecret(string(data))
			}),
			OnError(RequeueResponse()),
		),
		HandleResult(
			GetAction(
				types.NamespacedName{Namespace: instance.Namespace, Name: "serving-certs-ca-bundle"},
				kubeletCertsCM,
			),
			OnNotFound(Call(func() (ClientAction, error) {
				return nil, merrors.New("require kubelet-serving configmap is not found")
			})),
			OnContinue(manifests.CreateOrUpdateFactoryItemAction(
				&corev1.ConfigMap{},
				func() (runtime.Object, error) {
					return factory.PrometheusKubeletServingCABundle(kubeletCertsCM.Data["service-ca.crt"])
				},
				args,
			))),

		manifests.CreateOrUpdateFactoryItemAction(
			&corev1.Service{},
			func() (runtime.Object, error) {
				return factory.PrometheusService(instance.Name)
			},
			args),
		HandleResult(
			GetAction(
				types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
				prometheus,
			),
			OnNotFound(Call(r.createPrometheus(instance, factory, configSecret))),
			OnContinue(
				Call(func() (ClientAction, error) {
					expectedPrometheus, err := r.newPrometheusOperator(instance, factory, configSecret)

					if err != nil {
						return nil, merrors.Wrap(err, "error updating prometheus")
					}
					patch, err := r.patcher.Calculate(prometheus, expectedPrometheus)
					if err != nil {
						return nil, err
					}

					if patch.IsEmpty() {
						return nil, nil
					}

					r.patcher.SetLastAppliedAnnotation(expectedPrometheus)

					patch, err = r.patcher.Calculate(prometheus, expectedPrometheus)
					if err != nil {
						return nil, merrors.Wrap(err, "error creating patch")
					}

					if patch.IsEmpty() {
						return nil, nil
					}

					updateResult := &ExecResult{}

					return HandleResult(
						StoreResult(
							updateResult,
							UpdateWithPatchAction(prometheus, types.MergePatchType, patch.Patch),
						),
						OnError(
							Call(func() (ClientAction, error) {
								return UpdateStatusCondition(
									instance, instance.Status.Conditions, status.Condition{
										Type:    marketplacev1alpha1.ConditionError,
										Status:  corev1.ConditionFalse,
										Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
										Message: updateResult.Error(),
									}), nil
							})),
					), nil
				},
				))),
	}
}

func (r *ReconcileMeterBase) uninstallMetricState(
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
) []ClientAction {
	deployment, _ := factory.MetricStateDeployment()
	service, _ := factory.MetricStateService()
	sm, _ := factory.MetricStateServiceMonitor()

	return []ClientAction{
		HandleResult(
			GetAction(types.NamespacedName{Namespace: sm.Namespace, Name: sm.Name}, sm),
			OnContinue(DeleteAction(sm))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, service),
			OnContinue(DeleteAction(service))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: deployment.Namespace, Name: deployment.Name}, deployment),
			OnContinue(DeleteAction(deployment))),
	}
}

func (r *ReconcileMeterBase) uninstallPrometheus(
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
) []ClientAction {
	cm0, _ := factory.PrometheusServingCertsCABundle()
	secret0, _ := factory.PrometheusDatasources()
	secret1, _ := factory.PrometheusProxySecret()
	secret2, _ := factory.PrometheusHtpasswdSecret("foo")
	secret3, _ := factory.PrometheusRBACProxySecret()
	secrets := []*corev1.Secret{secret0, secret1, secret2, secret3}
	prom, _ := r.newPrometheusOperator(instance, factory, nil)
	service, _ := factory.PrometheusService(instance.Name)
	deployment, _ := factory.MetricStateDeployment()
	service2, _ := factory.MetricStateService()
	sm, _ := factory.MetricStateServiceMonitor()

	actions := []ClientAction{
		HandleResult(
			GetAction(
				types.NamespacedName{Namespace: cm0.Namespace, Name: cm0.Name}, cm0),
			OnContinue(DeleteAction(cm0))),
	}
	for _, sec := range secrets {
		actions = append(actions,
			HandleResult(
				GetAction(
					types.NamespacedName{Namespace: sec.Namespace, Name: sec.Name}, sec),
				OnContinue(DeleteAction(sec))))
	}

	return append(actions,
		HandleResult(
			GetAction(types.NamespacedName{Namespace: sm.Namespace, Name: sm.Name}, sm),
			OnContinue(DeleteAction(sm))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: service2.Namespace, Name: service2.Name}, deployment),
			OnContinue(DeleteAction(service2))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, service),
			OnContinue(DeleteAction(service))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: deployment.Namespace, Name: deployment.Name}, deployment),
			OnContinue(DeleteAction(deployment))),
		HandleResult(
			GetAction(types.NamespacedName{Namespace: prom.Namespace, Name: prom.Name}, prom),
			OnContinue(DeleteAction(prom))),
	)
}

func (r *ReconcileMeterBase) reconcilePrometheusService(
	instance *marketplacev1alpha1.MeterBase,
	service *corev1.Service,
) []ClientAction {
	newService := r.serviceForPrometheus(instance, 9090)
	return []ClientAction{
		HandleResult(
			GetAction(
				types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
				service,
			),
			OnNotFound(CreateAction(newService, CreateWithAddOwner(instance)))),
	}
}

func (r *ReconcileMeterBase) createPrometheus(
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
	configSecret *corev1.Secret,
) func() (ClientAction, error) {
	return func() (ClientAction, error) {
		newProm, err := r.newPrometheusOperator(instance, factory, configSecret)
		createResult := &ExecResult{}

		if err != nil {
			return nil, merrors.Wrap(err, "error creating prometheus")
		}

		return HandleResult(
			StoreResult(
				createResult, CreateAction(
					newProm,
					CreateWithAddOwner(instance),
					CreateWithPatch(r.patcher),
				)),
			OnError(
				Call(func() (ClientAction, error) {
					return UpdateStatusCondition(instance, instance.Status.Conditions, status.Condition{
						Type:    marketplacev1alpha1.ConditionError,
						Status:  corev1.ConditionFalse,
						Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
						Message: createResult.Error(),
					}), nil
				})),
			OnRequeue(
				UpdateStatusCondition(instance, instance.Status.Conditions, status.Condition{
					Type:    marketplacev1alpha1.ConditionInstalling,
					Status:  corev1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
					Message: "created prometheus",
				})),
		), nil
	}
}

func (r *ReconcileMeterBase) newPrometheusOperator(
	cr *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
	cfg *corev1.Secret,
) (*monitoringv1.Prometheus, error) {
	prom, err := factory.NewPrometheusDeployment(cr, cfg)

	if cr.Spec.Prometheus.Storage.Class == nil {
		defaultClass, err := utils.GetDefaultStorageClass(r.client)

		if err != nil {
			return prom, err
		}
		prom.Spec.Storage.VolumeClaimTemplate.Spec.StorageClassName = ptr.String(defaultClass)
	}

	return prom, err
}

// serviceForPrometheus function takes in a Prometheus object and returns a Service for that object.
func (r *ReconcileMeterBase) serviceForPrometheus(
	cr *marketplacev1alpha1.MeterBase,
	port int32) *corev1.Service {

	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"prometheus": cr.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "web",
					Port:       port,
					TargetPort: intstr.FromString("web"),
				},
			},
			ClusterIP: "None",
		},
	}
	return ser
}

func (r *ReconcileMeterBase) newBaseConfigMap(filename string, cr *marketplacev1alpha1.MeterBase) (*corev1.ConfigMap, error) {
	int, err := utils.LoadYAML(filename, corev1.ConfigMap{})
	if err != nil {
		return nil, err
	}

	cfg := (int).(*corev1.ConfigMap)
	cfg.Namespace = cr.Namespace
	cfg.Name = cr.Name

	return cfg, nil
}

// labelsForPrometheusOperator returns the labels for selecting the resources
// belonging to the given prometheus CR name.
func labelsForPrometheusOperator(name string) map[string]string {
	return map[string]string{"prometheus": name}
}

func labelsForServiceMonitor(name, namespace string) map[string]string {
	return map[string]string{
		"marketplace.redhat.com/metered":                  "true",
		"marketplace.redhat.com/deployed":                 "true",
		"marketplace.redhat.com/metered.kind":             "InternalServiceMonitor",
		"marketplace.redhat.com/serviceMonitor.Name":      name,
		"marketplace.redhat.com/serviceMonitor.Namespace": namespace,
	}
}
