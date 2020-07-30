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
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
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
				Do(r.uninstallServiceMonitors(instance)...),
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

	/// ---
	/// Install Objects
	/// ---

	prometheus := &monitoringv1.Prometheus{}
	if result, err := cc.Do(context.TODO(),
		Do(r.reconcilePrometheusOperator(instance, factory)...),
		Do(r.reconcilePrometheus(instance, prometheus, factory)...),
		Do(r.installServiceMonitors(instance)...),
		Do(r.installMetricStateDeployment(instance, factory)...),
	); !result.Is(Continue) {
		if err != nil {
			reqLogger.Error(err, "error in reconcile")
			return result.ReturnWithError(merrors.Wrap(err, "error creating prometheus"))
		}

		return result.Return()
	}

	// ----
	// Update our status
	// ----

	// Set status for prometheus

	prometheusStatefulset := &appsv1.StatefulSet{}
	if result, err := cc.Do(
		context.TODO(),
		HandleResult(
			GetAction(types.NamespacedName{
				Namespace: prometheus.Namespace,
				Name:      fmt.Sprintf("prometheus-%s", prometheus.Name),
			}, prometheusStatefulset),
			OnContinue(Call(func() (ClientAction, error) {
				updatedInstance := instance.DeepCopy()
				updatedInstance.Status.Replicas = &prometheusStatefulset.Status.CurrentReplicas
				updatedInstance.Status.UpdatedReplicas = &prometheusStatefulset.Status.UpdatedReplicas
				updatedInstance.Status.AvailableReplicas = &prometheusStatefulset.Status.ReadyReplicas
				updatedInstance.Status.UnavailableReplicas = ptr.Int32(
					prometheusStatefulset.Status.CurrentReplicas - prometheusStatefulset.Status.ReadyReplicas)

				if reflect.DeepEqual(updatedInstance.Status, instance.Status) {
					reqLogger.Info("prometheus statefulset status is up to date")
					return nil, nil
				}

				var action ClientAction = nil

				reqLogger.Info("statefulset status", "status", updatedInstance.Status)

				if updatedInstance.Status.Replicas != updatedInstance.Status.AvailableReplicas {
					reqLogger.Info("prometheus statefulset has not finished roll out",
						"replicas", updatedInstance.Status.Replicas,
						"available", updatedInstance.Status.AvailableReplicas)
					action = RequeueAfterResponse(30 * time.Second)
				}

				return HandleResult(
					UpdateAction(updatedInstance, UpdateStatusOnly(true)),
					OnContinue(action)), nil
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
		Status:  corev1.ConditionTrue,
		Reason:  marketplacev1alpha1.ReasonMeterBaseFinishInstall,
		Message: message,
	})); result.Is(Error) || result.Is(Requeue) {
		if err != nil {
			return result.ReturnWithError(merrors.Wrap(err, "error creating service monitor"))
		}

		return result.Return()
	}

	// Create meter reports ---
	// collect meter reports - list
	// check for last n exists - last 30 since time of creation
	// verify no gaps - 0-1, 2-3 - 1-2 is missing, then create 1-2
	// create any new reports necessary
	// use UTC for dates
	// requeue every 10 minutes
	// ex:
	//
	// I get a list of 0-1, 2-3, 3-4, 4-5 and my time is now 6, with a start of 0
	// I should create 1-2, and 5-6
	//
	// If my start is -1
	// -1-0 and 1-2, 5-6
	meterReportList := &marketplacev1alpha1.MeterReportList{}
	if result, err := cc.Do(
		context.TODO(),
		HandleResult(
			ListAction(meterReportList, client.InNamespace(request.Namespace)),
			OnContinue(Call(func() (ClientAction, error) {

				var meterReportNames []string
				for _, report := range meterReportList.Items {
					meterReportNames = append(meterReportNames, report.Name)
				}

				sort.Strings(meterReportNames)
				loc, _ := time.LoadLocation("UTC")

				/*  
					prune old reports
				*/

				for _,reportName := range meterReportNames {
					limit := time.Now().In(loc).AddDate(0, 0, -30)
					dateCreated, _ := r.retrieveCreatedDate(reportName)
					if dateCreated.Before(limit) {
						meterReportNames = utils.RemoveKey(meterReportNames,reportName)
						deleteReport := &marketplacev1alpha1.MeterReport{
							ObjectMeta: metav1.ObjectMeta{
								Name:      reportName,
								Namespace: request.Namespace,
							},
						}
						err := r.client.Delete(context.TODO(), deleteReport)
						if err != nil {
							reqLogger.Error(err, "Failed to delete MeterReport", "Resource", "Meter Report")
						}
					}
				}
				
				/*  
					fill in gaps of missing reports
				*/
				expectedCreatedDates := r.generateExpectedDates()
				foundCreatedDates := r.generateFoundCreatedDates(meterReportNames)

				// find the diff between the dates we expect and the dates found on the cluster
				diffs := utils.FindDiff(expectedCreatedDates, foundCreatedDates)
				for _,missingReportDateString := range diffs {
					fmt.Println("diff: ", missingReportDateString)

					missingReportName := r.newMeterReportNameFromString(missingReportDateString)
					missingReportStartDate,_ := time.Parse(utils.DATE_FORMAT, missingReportDateString)
					missingReportEndDate := missingReportStartDate.AddDate(0, 0, 1)

					missingMeterReport := r.newMeterReport(request.Namespace, missingReportStartDate, missingReportEndDate, missingReportName)
					err := r.client.Create(context.TODO(), missingMeterReport)
					if err != nil {
						reqLogger.Error(err, "error creating new report")
					}
				}

				/*
				    Create scheduled reports
				*/

				startTime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, loc)
				endTime := startTime.AddDate(0, 0, 1)
				newMeterReportName := r.newMeterReportNameFromDate(startTime)
				newMeterReport := r.newMeterReport(request.Namespace, startTime, endTime, newMeterReportName)
				err := r.client.Create(context.TODO(), newMeterReport)
				if err != nil {
					reqLogger.Error(err, "error creating new report")
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
	return reconcile.Result{}, nil
}

// configPath: /etc/config/prometheus.yml

type Images struct {
	ConfigmapReload string
	Server          string
}

type MeterbaseOpts struct {
	corev1.PullPolicy
}

func (r *ReconcileMeterBase) retrieveCreatedDate(reportName string) (time.Time, error) {
	dateString := strings.SplitN(reportName, "-", 3)[2:]
	return time.Parse(utils.DATE_FORMAT, strings.Join(dateString, ""))
}

func (r *ReconcileMeterBase) newMeterReportNameFromDate(date time.Time) string {
	prefix := "meter-report-"
	dateSuffix := strings.Join(strings.Fields(date.String())[:1], "")
	return fmt.Sprintf("%s%s", prefix, dateSuffix)
}

func (r *ReconcileMeterBase) newMeterReportNameFromString(dateString string) string {
	prefix := "meter-report-"
	dateSuffix := dateString
	return fmt.Sprintf("%s%s", prefix, dateSuffix)
}

func (r *ReconcileMeterBase) generateExpectedDates() []string {
	loc, _ := time.LoadLocation("UTC")
	// set start date
	startDate := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -30)
	fmt.Println("START DATE", startDate)

	// set end date
	endDate := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, loc)
	fmt.Println("END DATE", endDate)

	// loop through the range of dates we expect 
	var expectedCreatedDates []string
	for d := startDate; d.After(endDate) == false; d = d.AddDate(0, 0, 1) {
		expectedCreatedDates = append(expectedCreatedDates, d.Format(utils.DATE_FORMAT))
	}

	return expectedCreatedDates

}

func (r *ReconcileMeterBase) generateFoundCreatedDates(meterReportNames []string) []string {
	var foundCreatedDates []string
	for _, reportName := range meterReportNames {
		dateString := strings.SplitN(reportName, "-", 3)[2:]
		foundCreatedDates = append(foundCreatedDates, strings.Join(dateString, "")) 
	}
	return foundCreatedDates
}

func (r *ReconcileMeterBase) newMeterReport(namespace string, startTime time.Time, endTime time.Time, meterReportName string) *marketplacev1alpha1.MeterReport {
	return &marketplacev1alpha1.MeterReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meterReportName,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MeterReportSpec{
			StartTime: metav1.NewTime(startTime),
			EndTime:   metav1.NewTime(endTime),
			PrometheusService: &common.ServiceReference {
				Name:      "rhm-prometheus-meterbase",
				Namespace: "openshift-redhat-marketplace",
				TargetPort: intstr.IntOrString{
					StrVal: "web",
				},
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
	cm := &corev1.ConfigMap{}
	deployment := &appsv1.Deployment{}
	service := &corev1.Service{}

	args := manifests.CreateOrUpdateFactoryItemArgs{
		Owner:   instance,
		Patcher: r.patcher,
	}

	return []ClientAction{
		manifests.CreateOrUpdateFactoryItemAction(
			cm,
			func() (runtime.Object, error) {
				return factory.NewPrometheusOperatorCertsCABundle()
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			deployment,
			func() (runtime.Object, error) {
				return factory.NewPrometheusOperatorDeployment()
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			service,
			func() (runtime.Object, error) {
				return factory.NewPrometheusOperatorService()
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
	deployment, _ := factory.NewPrometheusOperatorDeployment()
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

func (r *ReconcileMeterBase) reconcilePrometheus(
	instance *marketplacev1alpha1.MeterBase,
	prometheus *monitoringv1.Prometheus,
	factory *manifests.Factory,
) []ClientAction {
	args := manifests.CreateOrUpdateFactoryItemArgs{
		Owner:   instance,
		Patcher: r.patcher,
	}

	dataSecret := &corev1.Secret{}
	kubeletCertsCM := &corev1.ConfigMap{}

	return []ClientAction{
		manifests.CreateOrUpdateFactoryItemAction(
			&corev1.ConfigMap{},
			func() (runtime.Object, error) {
				return factory.PrometheusServingCertsCABundle()
			},
			args,
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
				return factory.PrometheusHtpasswdSecret(string(dataSecret.Data["basicAuthSecret"]))
			}),
		manifests.CreateIfNotExistsFactoryItem(
			&corev1.Secret{},
			func() (runtime.Object, error) {
				return factory.PrometheusRBACProxySecret()
			}),
		HandleResult(
			GetAction(
				types.NamespacedName{Namespace: "openshift-config-managed", Name: "kubelet-serving-ca"},
				kubeletCertsCM,
			),
			OnNotFound(Call(func() (ClientAction, error) {
				return nil, merrors.New("require kubelet-serving configmap is not found")
			})),
			OnContinue(manifests.CreateOrUpdateFactoryItemAction(
				&corev1.ConfigMap{},
				func() (runtime.Object, error) {
					return factory.PrometheusKubeletServingCABundle(kubeletCertsCM.Data)
				},
				args,
			))),
		manifests.CreateOrUpdateFactoryItemAction(
			&corev1.Service{},
			func() (runtime.Object, error) {
				return factory.PrometheusService()
			},
			args),
		HandleResult(
			GetAction(
				types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
				prometheus,
			),
			OnNotFound(Call(r.createPrometheus(instance, factory))),
			OnContinue(Call(func() (ClientAction, error) {
				updatedPrometheus := prometheus.DeepCopy()
				expectedPrometheus, err := r.newPrometheusOperator(instance, factory)

				if err != nil {
					return nil, merrors.Wrap(err, "error updating prometheus")
				}

				updatedPrometheus.Spec = expectedPrometheus.Spec

				updateResult := &ExecResult{}

				return HandleResult(
					StoreResult(
						updateResult,
						UpdateWithPatchAction(prometheus, updatedPrometheus, r.patcher),
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
					OnRequeue(
						UpdateStatusCondition(instance, instance.Status.Conditions, status.Condition{
							Type:    marketplacev1alpha1.ConditionInstalling,
							Status:  corev1.ConditionTrue,
							Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
							Message: "updated prometheus",
						}))), nil
			}))),
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
	prom, _ := r.newPrometheusOperator(instance, factory)
	service, _ := factory.PrometheusService()

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
			GetAction(types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, service),
			OnContinue(DeleteAction(service))),
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
) func() (ClientAction, error) {
	return func() (ClientAction, error) {
		newProm, err := r.newPrometheusOperator(instance, factory)
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
) (*monitoringv1.Prometheus, error) {
	prom, err := factory.NewPrometheusDeployment()
	prom.Name = cr.Name
	prom.ObjectMeta.Name = cr.Name

	if err != nil {
		return prom, err
	}

	storageClass := ""
	if cr.Spec.Prometheus.Storage.Class == nil {
		foundDefaultClass, err := utils.GetDefaultStorageClass(r.client)

		if err != nil {
			return prom, err
		}

		storageClass = foundDefaultClass
	} else {
		storageClass = *cr.Spec.Prometheus.Storage.Class
	}

	pvc, err := utils.NewPersistentVolumeClaim(utils.PersistentVolume{
		ObjectMeta: &metav1.ObjectMeta{
			Name: "storage-volume",
		},
		StorageClass: &storageClass,
		StorageSize:  &cr.Spec.Prometheus.Storage.Size,
	})

	if err != nil {
		return prom, err
	}

	prom.Spec.Storage.VolumeClaimTemplate = pvc

	return prom, err
}

func (r *ReconcileMeterBase) installServiceMonitors(instance *marketplacev1alpha1.MeterBase) []ClientAction {
	reqLogger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	// find specific service monitor for kubelet
	openshiftKubeletMonitor := &monitoringv1.ServiceMonitor{}
	openshiftKubeStateMonitor := &monitoringv1.ServiceMonitor{}
	meteringKubeletMonitor := &monitoringv1.ServiceMonitor{}
	meteringKubeStateMonitor := &monitoringv1.ServiceMonitor{}

	return []ClientAction{
		Do(r.copyServiceMonitor(
			reqLogger,
			instance,
			types.NamespacedName{
				Namespace: "openshift-monitoring",
				Name:      "kubelet",
			},
			openshiftKubeletMonitor,
			types.NamespacedName{
				Namespace: instance.Namespace,
				Name:      "rhm-kubelet",
			},
			meteringKubeletMonitor,
		)...),
		Do(r.copyServiceMonitor(
			reqLogger,
			instance,
			types.NamespacedName{
				Namespace: "openshift-monitoring",
				Name:      "kube-state-metrics",
			},
			openshiftKubeStateMonitor,
			types.NamespacedName{
				Namespace: instance.Namespace,
				Name:      "rhm-kube-state-metrics",
			},
			meteringKubeStateMonitor,
		)...),
	}
}

func (r *ReconcileMeterBase) uninstallServiceMonitors(instance *marketplacev1alpha1.MeterBase) []ClientAction {
	sm1 := &monitoringv1.ServiceMonitor{}
	sm2 := &monitoringv1.ServiceMonitor{}

	return []ClientAction{
		HandleResult(GetAction(types.NamespacedName{
			Namespace: instance.Namespace,
			Name:      "rhm-kubelet",
		}, sm1), OnContinue(DeleteAction(sm1))),
		HandleResult(GetAction(types.NamespacedName{
			Namespace: instance.Namespace,
			Name:      "rhm-kubelet",
		}, sm2), OnContinue(DeleteAction(sm2))),
	}
}

func (r *ReconcileMeterBase) copyServiceMonitor(
	log logr.Logger,
	instance *marketplacev1alpha1.MeterBase,
	fromName types.NamespacedName,
	fromServiceMonitor *monitoringv1.ServiceMonitor,
	toName types.NamespacedName,
	toServiceMonitor *monitoringv1.ServiceMonitor,
) []ClientAction {
	return []ClientAction{
		HandleResult(
			GetAction(fromName, fromServiceMonitor),
			OnContinue(Call(func() (ClientAction, error) {
				newServiceMonitor := &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      toName.Name,
						Namespace: toName.Namespace,
						Labels:    labelsForServiceMonitor(fromServiceMonitor.Name, fromServiceMonitor.Namespace),
					},
					Spec: *fromServiceMonitor.Spec.DeepCopy(),
				}

				//newServiceMonitor.Spec.NamespaceSelector.MatchNames = []string{fromServiceMonitor.Namespace}
				newServiceMonitor.Spec.NamespaceSelector = monitoringv1.NamespaceSelector{
					MatchNames: []string{fromServiceMonitor.Namespace},
				}

				log.V(2).Info("cloning from", "obj", fromServiceMonitor, "spec", fromServiceMonitor.Spec)
				log.V(2).Info("created obj", "obj", newServiceMonitor, "spec", newServiceMonitor.Spec)

				return HandleResult(
					GetAction(
						types.NamespacedName{Name: newServiceMonitor.Name, Namespace: newServiceMonitor.Namespace},
						toServiceMonitor,
					),
					OnNotFound(CreateAction(newServiceMonitor, CreateWithAddOwner(instance))),
					OnContinue(
						UpdateWithPatchAction(
							toServiceMonitor,
							newServiceMonitor,
							r.patcher),
					),
				), nil
			})),
		),
	}
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
