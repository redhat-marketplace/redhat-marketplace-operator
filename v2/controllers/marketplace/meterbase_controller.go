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
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/operrors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	ctrl "sigs.k8s.io/controller-runtime"

	merrors "emperror.dev/errors"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	prom "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	DEFAULT_PROM_SERVER            = "prom/prometheus:v2.15.2"
	DEFAULT_CONFIGMAP_RELOAD       = "jimmidyson/configmap-reload:v0.3.0"
	RELATED_IMAGE_PROM_SERVER      = "RELATED_IMAGE_PROM_SERVER"
	RELATED_IMAGE_CONFIGMAP_RELOAD = "RELATED_IMAGE_CONFIGMAP_RELOAD"
)

// blank assignment to verify that ReconcileMeterBase implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterBaseReconciler{}

// MeterBaseReconciler reconciles a MeterBase object
type MeterBaseReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	CC     ClientCommandRunner

	cfg     *config.OperatorConfig
	factory *manifests.Factory
	patcher patch.Patcher
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

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *MeterBaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFn := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name:      "rhm-marketplaceconfig-meterbase",
					Namespace: "openshift-redhat-marketplace",
				}},
			}
		})

	namespacePredicate := predicates.NamespacePredicate(r.cfg.DeployedNamespace)

	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MeterBase{}).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &monitoringv1.Prometheus{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
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
			&source.Kind{Type: &appsv1.StatefulSet{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &corev1.Namespace{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: mapFn,
			}).Complete(r)
}

// Reconcile reads that state of the cluster for a MeterBase object and makes changes based on the state read
// and what is in the MeterBase.Spec
func (r *MeterBaseReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterBase")

	factory := r.factory
	cc := r.CC

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
		instance.Status.Conditions = status.Conditions{}
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
		Do(r.verifyPVCSize(reqLogger, instance, factory, prometheus)...),
		Do(r.recyclePrometheusPods(reqLogger, instance, factory, prometheus)...),
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
				reqLogger.Info("can't find prometheus statefulset, requeuing")
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
	if result, err := cc.Do(context.TODO(), UpdateStatusCondition(instance, &instance.Status.Conditions, status.Condition{
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

				reqLogger.Info("report dates", "expected", expectedCreatedDates, "found", foundCreatedDates, "min", minDate)
				err = r.createReportIfNotFound(expectedCreatedDates, foundCreatedDates, request, instance)

				if err != nil {
					return nil, err
				}

				return nil, nil
			})),
			OnNotFound(Call(func() (ClientAction, error) {
				reqLogger.Info("can't find meter report list, requeuing")
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

func (r *MeterBaseReconciler) createReportIfNotFound(expectedCreatedDates []string, foundCreatedDates []string, request reconcile.Request, instance *marketplacev1alpha1.MeterBase) error {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	// find the diff between the dates we expect and the dates found on the cluster and create any missing reports
	missingReports := utils.FindDiff(expectedCreatedDates, foundCreatedDates)
	for _, missingReportDateString := range missingReports {
		missingReportName := r.newMeterReportNameFromString(missingReportDateString)
		missingReportStartDate, _ := time.Parse(utils.DATE_FORMAT, missingReportDateString)
		missingReportEndDate := missingReportStartDate.AddDate(0, 0, 1)

		missingMeterReport := r.newMeterReport(request.Namespace, missingReportStartDate, missingReportEndDate, missingReportName, instance, promServiceName)
		err := r.Client.Create(context.TODO(), missingMeterReport)
		if err != nil {
			return err
		}
		reqLogger.Info("Created Missing Report", "Resource", missingReportName)
	}

	return nil
}

func (r *MeterBaseReconciler) removeOldReports(meterReportNames []string, loc *time.Location, dateRange int, request reconcile.Request) ([]string, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
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
			err := r.Client.Delete(context.TODO(), deleteReport)
			if err != nil {
				return nil, err
			}
		}
	}

	return meterReportNames, nil
}

func (r *MeterBaseReconciler) sortMeterReports(meterReportList *marketplacev1alpha1.MeterReportList) []string {

	var meterReportNames []string
	for _, report := range meterReportList.Items {
		meterReportNames = append(meterReportNames, report.Name)
	}

	sort.Strings(meterReportNames)
	return meterReportNames
}

func (r *MeterBaseReconciler) retrieveCreatedDate(reportName string) (time.Time, error) {
	splitStr := strings.SplitN(reportName, "-", 3)

	if len(splitStr) != 3 {
		return time.Now(), errors.New("failed to get date")
	}

	dateString := splitStr[2:]
	return time.Parse(utils.DATE_FORMAT, strings.Join(dateString, ""))
}

func (r *MeterBaseReconciler) newMeterReportNameFromDate(date time.Time) string {
	dateSuffix := strings.Join(strings.Fields(date.String())[:1], "")
	return fmt.Sprintf("%s%s", utils.METER_REPORT_PREFIX, dateSuffix)
}

func (r *MeterBaseReconciler) newMeterReportNameFromString(dateString string) string {
	dateSuffix := dateString
	return fmt.Sprintf("%s%s", utils.METER_REPORT_PREFIX, dateSuffix)
}

func (r *MeterBaseReconciler) generateFoundCreatedDates(meterReportNames []string) ([]string, error) {
	reqLogger := r.Log.WithValues("func", "generateFoundCreatedDates")
	var foundCreatedDates []string
	for _, reportName := range meterReportNames {
		splitStr := strings.SplitN(reportName, "-", 3)

		if len(splitStr) != 3 {
			reqLogger.Info("meterreport name was irregular", "name", reportName)
			continue
		}

		dateString := splitStr[2:]
		foundCreatedDates = append(foundCreatedDates, strings.Join(dateString, ""))
	}
	return foundCreatedDates, nil
}

func (r *MeterBaseReconciler) generateExpectedDates(endTime time.Time, loc *time.Location, dateRange int, minDate time.Time) []string {
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

func (r *MeterBaseReconciler) newMeterReport(namespace string, startTime time.Time, endTime time.Time, meterReportName string, instance *marketplacev1alpha1.MeterBase, prometheusServiceName string) *marketplacev1alpha1.MeterReport {
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

func (r *MeterBaseReconciler) reconcilePrometheusSubscription(
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
					CreateWithAddController(instance),
				),
			)),
	}
}

func (r *MeterBaseReconciler) reconcilePrometheusOperator(
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
) []ClientAction {
	reqLogger := r.Log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	cm := &corev1.ConfigMap{}
	deployment := &appsv1.Deployment{}
	service := &corev1.Service{}

	args := manifests.CreateOrUpdateFactoryItemArgs{
		Owner:   instance,
		Patcher: r.patcher,
	}

	watchNamespace, _ := getWatchNamespace()

	return []ClientAction{
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
				nsValues := strings.Split(watchNamespace, ",")
				sort.Strings(nsValues)
				reqLogger.Info("found namespaces", "ns", nsValues)
				return factory.NewPrometheusOperatorDeployment(nsValues)
			},
			args,
		),
	}
}

func (r *MeterBaseReconciler) installMetricStateDeployment(
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

func (r *MeterBaseReconciler) uninstallPrometheusOperator(
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

func (r *MeterBaseReconciler) reconcileAdditionalConfigSecret(
	cc ClientCommandRunner,
	instance *marketplacev1alpha1.MeterBase,
	prometheus *monitoringv1.Prometheus,
	factory *manifests.Factory,
	additionalConfigSecret *corev1.Secret,
) []ClientAction {
	reqLogger := r.Log.WithValues("func", "reconcileAdditionalConfigSecret", "Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	openshiftKubeletMonitor := &monitoringv1.ServiceMonitor{}
	openshiftKubeStateMonitor := &monitoringv1.ServiceMonitor{}
	metricStateMonitor := &monitoringv1.ServiceMonitor{}
	secretsInNamespace := &corev1.SecretList{}

	sm, err := factory.MetricStateServiceMonitor()

	if err != nil {
		reqLogger.Error(err, "error getting metric state")
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

			cfgGen := prom.NewConfigGenerator(reqLogger)

			basicAuthSecrets, err := prom.LoadBasicAuthSecrets(r.Client, sMons, prometheus.Spec.RemoteRead, prometheus.Spec.RemoteWrite, prometheus.Spec.APIServerConfig, secretsInNamespace)
			if err != nil {
				return nil, err
			}

			bearerTokens, err := prom.LoadBearerTokensFromSecrets(r.Client, sMons)
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
				OnNotFound(CreateAction(sec, CreateWithAddController(instance))),
				OnContinue(Call(func() (ClientAction, error) {

					if reflect.DeepEqual(additionalConfigSecret.Data, sec.Data) {
						return nil, nil
					}

					return UpdateAction(sec), nil
				}))), nil
		}),
	}
}

func (r *MeterBaseReconciler) reconcilePrometheus(
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
				types.NamespacedName{Namespace: "openshift-monitoring", Name: "kubelet-serving-ca-bundle"},
				kubeletCertsCM,
			),
			OnNotFound(Call(func() (ClientAction, error) {
				return nil, merrors.New("require kubelet-serving configmap is not found")
			})),
			OnContinue(manifests.CreateOrUpdateFactoryItemAction(
				&corev1.ConfigMap{},
				func() (runtime.Object, error) {
					return factory.PrometheusKubeletServingCABundle(kubeletCertsCM.Data["ca-bundle.crt"])
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

					if orig, _ := r.patcher.GetOriginalConfiguration(prometheus); orig == nil {
						data, _ := r.patcher.GetModifiedConfiguration(prometheus, false)
						r.patcher.SetOriginalConfiguration(prometheus, data)
					}

					patch, err := r.patcher.Calculate(prometheus, expectedPrometheus)
					if err != nil {
						return nil, merrors.Wrap(err, "error creating patch")
					}

					if patch.IsEmpty() {
						return nil, nil
					}

					err = r.patcher.SetLastAppliedAnnotation(expectedPrometheus)
					if err != nil {
						return nil, merrors.Wrap(err, "error creating patch")
					}

					patch, err = r.patcher.Calculate(prometheus, expectedPrometheus)
					if err != nil {
						return nil, merrors.Wrap(err, "error creating patch")
					}

					if patch.IsEmpty() {
						return nil, nil
					}

					jsonPatch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(patch.Original, patch.Modified, patch.Current)
					if err != nil {
						return nil, merrors.Wrap(err, "Failed to generate merge patch")
					}

					updateResult := &ExecResult{}

					return HandleResult(
						StoreResult(
							updateResult,
							UpdateWithPatchAction(prometheus, types.MergePatchType, jsonPatch),
						),
						OnError(
							Call(func() (ClientAction, error) {
								return UpdateStatusCondition(
									instance, &instance.Status.Conditions, status.Condition{
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

func (r *MeterBaseReconciler) recyclePrometheusPods(
	log logr.Logger,
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
	prometheusDeployment *monitoringv1.Prometheus,
) []ClientAction {
	pvcs := &corev1.PersistentVolumeClaimList{}

	return []ClientAction{
		Call(func() (ClientAction, error) {
			return ListAction(pvcs, client.MatchingLabels{"prometheus": prometheusDeployment.Name}), nil
		}),
		Call(func() (ClientAction, error) {
			if len(pvcs.Items) == 0 {
				log.Info("no pvcs found")
				return nil, nil
			}

			for _, item := range pvcs.Items {
				for _, cond := range item.Status.Conditions {
					if cond.Type == corev1.PersistentVolumeClaimFileSystemResizePending {
						name := strings.Replace(item.Name, "prometheus-rhm-marketplaceconfig-meterbase-db-", "", -1)
						log.Info("volume pending resize, restarting pod", "name", name)
						pod := &corev1.Pod{}

						return HandleResult(
							GetAction(types.NamespacedName{Name: name, Namespace: instance.Namespace}, pod),
							OnContinue(
								HandleResult(
									DeleteAction(pod),
									OnAny(RequeueAfterResponse(10*time.Second))),
							),
						), nil
					}
				}
			}

			return nil, nil
		}),
	}
}

func (r *MeterBaseReconciler) verifyPVCSize(
	log logr.Logger,
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
	prometheusDeployment *monitoringv1.Prometheus,
) []ClientAction {
	storageClass := &storagev1.StorageClass{}
	pvcs := &corev1.PersistentVolumeClaimList{}

	return []ClientAction{
		Call(func() (ClientAction, error) {
			return ListAction(pvcs, client.MatchingLabels{"prometheus": prometheusDeployment.Name}), nil
		}),
		HandleResult(
			Call(func() (ClientAction, error) {
				if len(pvcs.Items) == 0 {
					log.Info("no pvcs found")
					return nil, nil
				}

				for _, item := range pvcs.Items {
					if item.Spec.StorageClassName == nil {
						log.Info("storage class is nil, skipping")
						continue
					}

					log.Info("storage class lookup", "name", *item.Spec.StorageClassName)
					return GetAction(types.NamespacedName{Name: *item.Spec.StorageClassName}, storageClass), nil
				}

				return nil, nil
			}),
			OnContinue(Call(func() (ClientAction, error) {
				if len(pvcs.Items) == 0 {
					return nil, nil
				}

				if storageClass == nil {
					return nil, nil
				}

				if storageClass.AllowVolumeExpansion == nil ||
					(storageClass.AllowVolumeExpansion != nil && *storageClass.AllowVolumeExpansion != true) {
					log.Info("storage class does not allow for expansion", "storageClass", storageClass.String())
					return nil, nil
				}

				if prometheusDeployment.Spec.Storage == nil ||
					prometheusDeployment.Spec.Storage.VolumeClaimTemplate.Spec.Resources.Requests.Storage() == nil {
					log.Info("prometheusDeployment Storage not defined")
					return nil, nil
				}

				actions := []ClientAction{}
				promDefinedStorage := prometheusDeployment.Spec.Storage.VolumeClaimTemplate.Spec.Resources.Requests.Storage()

				for _, item := range pvcs.Items {
					switch item.Spec.Resources.Requests.Storage().Cmp(*promDefinedStorage) {
					case 0:
						fallthrough
					case 1:
						log.Info("pvc size is not different", "oldSize", item.Spec.Resources.Requests.Storage().String(), "newSize", promDefinedStorage.String())
						continue
					default:
					}

					localItem := item.DeepCopy()
					log.Info("pvc size is different", "oldSize", localItem.Spec.Resources.Requests.Storage().String(), "newSize", promDefinedStorage.String())
					localItem.Spec.Resources.Requests = corev1.ResourceList{
						corev1.ResourceStorage: *promDefinedStorage,
					}
					actions = append(actions, UpdateAction(localItem))
				}

				return Do(actions...), nil
			})),
		),
	}
}

func (r *MeterBaseReconciler) uninstallMetricState(
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

func (r *MeterBaseReconciler) uninstallPrometheus(
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

func (r *MeterBaseReconciler) reconcilePrometheusService(
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
			OnNotFound(CreateAction(newService, CreateWithAddController(instance)))),
	}
}

func (r *MeterBaseReconciler) createPrometheus(
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
	configSecret *corev1.Secret,
) func() (ClientAction, error) {
	return func() (ClientAction, error) {
		newProm, err := r.newPrometheusOperator(instance, factory, configSecret)
		createResult := &ExecResult{}

		if err != nil {
			if merrors.Is(err, operrors.DefaultStorageClassNotFound) {
				return UpdateStatusCondition(instance, &instance.Status.Conditions, status.Condition{
					Type:    marketplacev1alpha1.ConditionError,
					Status:  corev1.ConditionFalse,
					Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
					Message: err.Error(),
				}), nil
			}

			return nil, merrors.Wrap(err, "error creating prometheus")
		}

		return HandleResult(
			StoreResult(
				createResult, CreateAction(
					newProm,
					CreateWithAddController(instance),
					CreateWithPatch(r.patcher),
				)),
			OnError(
				Call(func() (ClientAction, error) {
					return UpdateStatusCondition(instance, &instance.Status.Conditions, status.Condition{
						Type:    marketplacev1alpha1.ConditionError,
						Status:  corev1.ConditionFalse,
						Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
						Message: createResult.Error(),
					}), nil
				})),
			OnRequeue(
				UpdateStatusCondition(instance, &instance.Status.Conditions, status.Condition{
					Type:    marketplacev1alpha1.ConditionInstalling,
					Status:  corev1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
					Message: "created prometheus",
				})),
		), nil
	}
}

func (r *MeterBaseReconciler) newPrometheusOperator(
	cr *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
	cfg *corev1.Secret,
) (*monitoringv1.Prometheus, error) {
	prom, err := factory.NewPrometheusDeployment(cr, cfg)

	factory.SetOwnerReference(cr, prom)

	if cr.Spec.Prometheus.Storage.Class == nil {
		defaultClass, err := utils.GetDefaultStorageClass(r.Client)

		if err != nil {
			return prom, err
		}
		prom.Spec.Storage.VolumeClaimTemplate.Spec.StorageClassName = ptr.String(defaultClass)
	}

	return prom, err
}

// serviceForPrometheus function takes in a Prometheus object and returns a Service for that object.
func (r *MeterBaseReconciler) serviceForPrometheus(
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

func (r *MeterBaseReconciler) newBaseConfigMap(filename string, cr *marketplacev1alpha1.MeterBase) (*corev1.ConfigMap, error) {
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
