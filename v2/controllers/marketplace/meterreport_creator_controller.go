package marketplace

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	merrors "emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
)

type MeterReportCreatorReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	CC     ClientCommandRunner
	cfg    *config.OperatorConfig
}

func (r *MeterReportCreatorReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log

	// Fetch the MeterBase instance
	instance := &marketplacev1alpha1.MeterBase{}
	result, _ := r.CC.Do(
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

	if !instance.Spec.Enabled {
		reqLogger.Info("metering is disabled")
		return reconcile.Result{}, nil
	}

	installingCond := instance.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionInstalling)

	if installingCond == nil || !(installingCond.IsFalse() && installingCond.Reason == marketplacev1alpha1.ReasonMeterBaseFinishInstall) {
		reqLogger.Info("metering is not finished installing", "installing", installingCond, "meterbase", instance)
		return reconcile.Result{}, nil
	}

	meterDefinitionList := &marketplacev1beta1.MeterDefinitionList{}
	var categoryList []string
	if result, err := r.CC.Do(
		context.TODO(),
		HandleResult(
			ListAction(meterDefinitionList),
			OnContinue(Call(func() (ClientAction, error) {
				categoryList = getCategoriesFromMeterDefinitions(meterDefinitionList.Items)
				return nil, nil
			})),
			OnNotFound(Call(func() (ClientAction, error) {
				reqLogger.Info("can't find meter definition list, requeuing")
				return ReturnFinishedResult(), nil
			})),
		),
	); result.Is(Error) || result.Is(Requeue) {
		if err != nil {
			return result.ReturnWithError(merrors.Wrap(err, "error listing meter definitions"))
		}

		return result.Return()
	}

	meterReportList := &marketplacev1alpha1.MeterReportList{}
	if result, err := r.CC.Do(
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
				for _, category := range categoryList {
					labels := make(map[string]string)
					labels["marketplace.redhat.com/category"] = category

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

					missingReports := r.findMissingReportsForCategory(expectedCreatedDates, foundCreatedDates)
					err = r.createMissingReports(missingReports, request, instance, metav1.LabelSelector{MatchLabels: labels}, category)
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			})),
			OnNotFound(Call(func() (ClientAction, error) {
				reqLogger.Info("can't find meter report list, requeuing")
				return ReturnFinishedResult(), nil
			})),
		),
	); result.Is(Error) || result.Is(Requeue) {
		if err != nil {
			return result.ReturnWithError(merrors.Wrap(err, "error creating service monitor"))
		}

		return result.Return()
	}

	return reconcile.Result{}, nil
}

func (r *MeterReportCreatorReconciler) Inject(injector mktypes.Injectable) {
	injector.SetCustomFields(r)
}

func (r *MeterReportCreatorReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

func (r *MeterReportCreatorReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.CC = ccp
	return nil
}

type CheckMeterReports event.GenericEvent

func (r *MeterReportCreatorReconciler) SetupWithManager(
	mgr ctrl.Manager,
	doneChannel <-chan struct{},
) error {
	events := make(chan event.GenericEvent)
	ticker := time.NewTicker(r.cfg.ReportController.PollTime)

	go func() {
		defer close(events)
		defer ticker.Stop()

		meta := &metav1.ObjectMeta{
			Name:      utils.METERBASE_NAME,
			Namespace: r.cfg.DeployedNamespace,
		}

		for {
			select {
			case <-doneChannel:
				return
			case <-ticker.C:
				events <- event.GenericEvent(CheckMeterReports{
					Meta: meta,
				})
			}
		}
	}()

	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MeterReport{}).
		Watches(
			&source.Channel{Source: events},
			&handler.EnqueueRequestForObject{}).
		Complete(r)
}

func (r *MeterReportCreatorReconciler) findMissingReportsForCategory(expectedCreatedDates []string, foundCreatedDates []string) []string {
	// find the diff between the dates we expect and the dates found on the cluster and create any missing reports
	missingReports := utils.FindDiff(expectedCreatedDates, foundCreatedDates)
	return missingReports
}

func (r *MeterReportCreatorReconciler) createMissingReports(missingReports []string, request reconcile.Request, instance *marketplacev1alpha1.MeterBase, labelSelector metav1.LabelSelector, category string) error {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	for _, missingReportDateString := range missingReports {
		missingReportName, nameErr := r.newMeterReportNameFromString(category, missingReportDateString)
		if nameErr != nil {
			return nameErr
		}
		missingReportStartDate, _ := time.Parse(utils.DATE_FORMAT, missingReportDateString)
		missingReportEndDate := missingReportStartDate.AddDate(0, 0, 1)

		missingMeterReport := r.newMeterReport(request.Namespace, missingReportStartDate, missingReportEndDate, missingReportName, instance, promServiceName, labelSelector, category)
		err := r.Client.Create(context.TODO(), missingMeterReport)
		if err != nil {
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
			err := r.Client.Delete(context.TODO(), deleteReport)
			if err != nil {
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
	splitAll := strings.SplitN(reportName, "-", -1)
	if len(splitAll) == 5 {
		splitStr := strings.SplitN(reportName, "-", 3)
		dateString := splitStr[2:]
		return time.Parse(utils.DATE_FORMAT, strings.Join(dateString, ""))
	} else if len(splitAll) == 4 {
		splitStr := strings.SplitN(reportName, "-", 4)
		dateString := splitStr[:3]
		return time.Parse(utils.DATE_FORMAT, strings.Join(dateString, "-"))
	}
	return time.Now(), errors.New("failed to get date")
}

func processCategoryString(category string) string {
	// Make a Regex to say we only want letters and numbers for category name
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		log.Fatal(err)
	}
	return reg.ReplaceAllString(category, "")
}

func (r *MeterReportCreatorReconciler) newMeterReportNameFromDate(category string, date time.Time) string {
	dateSuffix := strings.Join(strings.Fields(date.String())[:1], "")
	return fmt.Sprintf("%s-%s", dateSuffix, processCategoryString(category))
}

func (r *MeterReportCreatorReconciler) newMeterReportNameFromString(category string, dateString string) (string, error) {
	dateSuffix := dateString
	var reportName string
	if category == "" {
		// for meter definitions without category it creates report in old forma name (meter-report-[date])
		reportName = strings.ToLower(fmt.Sprintf("%s%s", utils.METER_REPORT_PREFIX, dateSuffix))
	} else {
		// for meter definition with category meter report name contains category ([date]-[category label])
		reportName = strings.ToLower(fmt.Sprintf("%s-%s", dateSuffix, processCategoryString(category)))
	}
	if len(reportName) > 64 {
		return reportName, errors.New("report name must be no more than 63 characters")
	}
	return reportName, nil
}

func (r *MeterReportCreatorReconciler) generateFoundCreatedDates(meterReportNames []string) ([]string, error) {
	reqLogger := r.Log.WithValues("func", "generateFoundCreatedDates")
	var foundCreatedDates []string
	for _, reportName := range meterReportNames {
		splitStr := strings.SplitN(reportName, "-", 4)

		if len(splitStr) != 4 {
			reqLogger.Info("meterreport name was irregular", "name", reportName)
			continue
		}

		dateString := splitStr[3:]
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
	for d := startDate; d.After(endDate) == false; d = d.AddDate(0, 0, 1) {
		expectedCreatedDates = append(expectedCreatedDates, d.Format(utils.DATE_FORMAT))
	}

	return expectedCreatedDates
}

func (r *MeterReportCreatorReconciler) newMeterReport(namespace string, startTime time.Time, endTime time.Time, meterReportName string, instance *marketplacev1alpha1.MeterBase, prometheusServiceName string, labelSelector metav1.LabelSelector, category string) *marketplacev1alpha1.MeterReport {
	return &marketplacev1alpha1.MeterReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meterReportName,
			Namespace: namespace,
			Annotations: map[string]string{
				"marketplace.redhat.com/version": version.Version,
			},
		},
		Spec: marketplacev1alpha1.MeterReportSpec{
			StartTime:     metav1.NewTime(startTime),
			EndTime:       metav1.NewTime(endTime),
			LabelSelector: labelSelector,
			Category:      category,
			PrometheusService: &common.ServiceReference{
				Name:       prometheusServiceName,
				Namespace:  instance.Namespace,
				TargetPort: intstr.FromString("rbac"),
			},
		},
	}
}
