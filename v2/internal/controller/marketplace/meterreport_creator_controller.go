// Copyright 2021 IBM Corp.
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
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type MeterReportCreatorReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	Cfg    *config.OperatorConfig
}

// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=meterreports,verbs=get;list;watch;create;delete

func (r *MeterReportCreatorReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log

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

	if !instance.Spec.Enabled {
		reqLogger.Info("metering is disabled")
		return reconcile.Result{}, nil
	}

	installingCond := instance.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionInstalling)

	if installingCond == nil || !(installingCond.IsFalse() && installingCond.Reason == marketplacev1alpha1.ReasonMeterBaseFinishInstall) {
		reqLogger.Info("metering is not finished installing", "installing", installingCond, "meterbase", instance)
		return reconcile.Result{}, nil
	}

	meterReportList := &marketplacev1alpha1.MeterReportList{}

	if err := r.Client.List(context.TODO(), meterReportList, client.InNamespace(request.Namespace)); err != nil {
		return reconcile.Result{}, nil
	}

	loc := time.UTC
	dateRangeInDays := -90

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
		return reconcile.Result{}, err
	}

	reqLogger.Info("report dates", "expected", expectedCreatedDates, "found", foundCreatedDates, "min", minDate)

	// Create the report with the active/to-be userWorkloadMonitoringEnabled state, regardless of transition state
	userWorkloadMonitoringEnabled := true
	if err := r.createReportIfNotFound(expectedCreatedDates, foundCreatedDates, request, instance, userWorkloadMonitoringEnabled); err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{}, nil
}

type CheckMeterReports event.GenericEvent

func (r *MeterReportCreatorReconciler) SetupWithManager(
	mgr ctrl.Manager,
	doneChannel <-chan struct{},
) error {
	events := make(chan event.GenericEvent)
	ticker := time.NewTicker(r.Cfg.ReportController.PollTime)

	go func() {
		defer close(events)
		defer ticker.Stop()

		meta := &marketplacev1alpha1.MeterBase{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.METERBASE_NAME,
				Namespace: r.Cfg.DeployedNamespace,
			},
		}

		for {
			events <- event.GenericEvent(CheckMeterReports{
				Object: meta,
			})

			select {
			case <-doneChannel:
				return
			case <-ticker.C:
			}
		}
	}()

	return ctrl.NewControllerManagedBy(mgr).
		Named("meterreport-creator").
		For(&marketplacev1alpha1.MeterBase{},
			builder.WithPredicates(
				predicate.Funcs{
					CreateFunc: func(e event.CreateEvent) bool {
						return false
					},
					UpdateFunc: func(e event.UpdateEvent) bool {
						return false
					},
					DeleteFunc: func(e event.DeleteEvent) bool {
						return false
					},
					GenericFunc: func(e event.GenericEvent) bool {
						return true
					},
				})).
		WatchesRawSource(
			source.Channel(
				events,
				&handler.EnqueueRequestForObject{},
			)).
		Complete(r)
}

func (r *MeterReportCreatorReconciler) newMeterReportNameFromString(dateString string) string {
	dateSuffix := dateString
	return fmt.Sprintf("%s%s", utils.METER_REPORT_PREFIX, dateSuffix)
}

func (r *MeterReportCreatorReconciler) createReportIfNotFound(expectedCreatedDates []string, foundCreatedDates []string, request reconcile.Request, instance *marketplacev1alpha1.MeterBase, userWorkloadMonitoringEnabled bool) error {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	// find the diff between the dates we expect and the dates found on the cluster and create any missing reports
	missingReports := utils.FindDiff(expectedCreatedDates, foundCreatedDates)
	for _, missingReportDateString := range missingReports {
		missingReportName := r.newMeterReportNameFromString(missingReportDateString)
		missingReportStartDate, _ := time.Parse(utils.DATE_FORMAT, missingReportDateString)
		missingReportEndDate := missingReportStartDate.AddDate(0, 0, 1)

		missingMeterReport := r.newMeterReport(request.Namespace, missingReportStartDate, missingReportEndDate, missingReportName, instance, userWorkloadMonitoringEnabled)
		if err := r.Client.Create(context.TODO(), missingMeterReport); err != nil {
			return err
		}
		reqLogger.Info("Created Missing Report", "Resource", missingReportName)
	}

	return nil
}

func (r *MeterReportCreatorReconciler) removeOldReports(meterReportNames []string, loc *time.Location, dateRange int, request reconcile.Request) ([]string, error) {
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
			if err := r.Client.Delete(context.TODO(), deleteReport); err != nil {
				return nil, err
			}
		}
	}

	return meterReportNames, nil
}

func (r *MeterReportCreatorReconciler) sortMeterReports(meterReportList *marketplacev1alpha1.MeterReportList) []string {
	var meterReportNames []string
	for _, report := range meterReportList.Items {
		meterReportNames = append(meterReportNames, report.Name)
	}

	sort.Strings(meterReportNames)
	return meterReportNames
}

func (r *MeterReportCreatorReconciler) retrieveCreatedDate(reportName string) (time.Time, error) {
	splitStr := strings.SplitN(reportName, "-", 3)

	if len(splitStr) != 3 {
		return time.Now(), errors.New("failed to get date")
	}

	dateString := splitStr[2:]
	return time.Parse(utils.DATE_FORMAT, strings.Join(dateString, ""))
}

func (r *MeterReportCreatorReconciler) generateFoundCreatedDates(meterReportNames []string) ([]string, error) {
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

func (r *MeterReportCreatorReconciler) generateExpectedDates(endTime time.Time, loc *time.Location, dateRange int, minDate time.Time) []string {
	// set start date
	startDate := utils.TruncateTime(endTime, loc).AddDate(0, 0, dateRange)

	if minDate.After(startDate) {
		startDate = utils.TruncateTime(minDate, loc)
	}

	// set end date
	endDate := utils.TruncateTime(endTime, loc)

	// loop through the range of dates we expect
	var expectedCreatedDates []string
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		expectedCreatedDates = append(expectedCreatedDates, d.Format(utils.DATE_FORMAT))
	}

	return expectedCreatedDates
}

func (r *MeterReportCreatorReconciler) newMeterReport(
	namespace string,
	startTime time.Time,
	endTime time.Time,
	meterReportName string,
	instance *marketplacev1alpha1.MeterBase,
	userWorkloadMonitoringEnabled bool,
) *marketplacev1alpha1.MeterReport {

	var promService *common.ServiceReference

	if userWorkloadMonitoringEnabled {
		promService = &common.ServiceReference{
			Name:       utils.OPENSHIFT_MONITORING_THANOS_QUERIER_SERVICE_NAME,
			Namespace:  utils.OPENSHIFT_MONITORING_NAMESPACE,
			TargetPort: intstr.FromString("web"),
		}
	} else {
		promService = &common.ServiceReference{
			Name:       utils.METERBASE_PROMETHEUS_SERVICE_NAME,
			Namespace:  instance.Namespace,
			TargetPort: intstr.FromString("rbac"),
		}
	}

	mreport := &marketplacev1alpha1.MeterReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meterReportName,
			Namespace: namespace,
			Annotations: map[string]string{
				"marketplace.redhat.com/version": version.Version,
			},
		},
		Spec: marketplacev1alpha1.MeterReportSpec{
			StartTime:         metav1.NewTime(startTime),
			EndTime:           metav1.NewTime(endTime),
			PrometheusService: promService,
		},
	}

	gvk, err := apiutil.GVKForObject(instance, r.Scheme)
	if err != nil {
		return nil
	}

	// create owner ref object
	ref := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               instance.GetName(),
		UID:                instance.GetUID(),
		BlockOwnerDeletion: pointer.BoolPtr(false),
		Controller:         pointer.BoolPtr(false),
	}

	mreport.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{ref})

	if instance.Spec.IsDataServiceEnabled() {
		mreport.Spec.ExtraArgs = append(mreport.Spec.ExtraArgs, "--uploadTargets=data-service")
		return mreport
	}

	return mreport
}
