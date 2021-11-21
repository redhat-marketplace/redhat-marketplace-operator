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

package reporter

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcileTask struct {
	logger    logr.Logger
	K8SClient client.Client
	Config    *Config
	K8SScheme *runtime.Scheme
	Namespace
	recorder record.EventRecorder

	NewTask   func(ctx context.Context, reportName ReportName, taskConfig *Config) (TaskRun, error)
	NewUpload func(ctx context.Context, config *Config, namespace Namespace) (UploadRun, error)
}

func (r *ReconcileTask) Run(ctx context.Context) error {
	reportErr := r.report(ctx)
	uploadErr := r.upload(ctx)

	err := errors.Combine(reportErr, uploadErr)
	if err != nil {
		terr := r.recordTaskError(ctx, err)
		if terr != nil {
			logger.Error(terr, "error recording error to events")
		}

		return err
	}

	return nil
}

func (r *ReconcileTask) report(ctx context.Context) error {
	logger.Info("reconcile report start")
	meterReports := marketplacev1alpha1.MeterReportList{}
	err := r.K8SClient.List(ctx, &meterReports, client.InNamespace(r.Namespace))

	if err != nil {
		return err
	}

	var errs []error

	meterReportsToRunNames := []string{}
	meterReportsToRun := []*marketplacev1alpha1.MeterReport{}

	// Run reports and upload to data service
	for i := range meterReports.Items {
		report := meterReports.Items[i]
		if r.CanRunReportTask(ctx, report) {
			meterReportsToRun = append(meterReportsToRun, &report)
			meterReportsToRunNames = append(meterReportsToRunNames, report.Name)
		}
	}

	logger.Info("report: meter reports ready to run", "reports", strings.Join(meterReportsToRunNames, ", "))

	for _, report := range meterReportsToRun {
		logger.Info("report: running report", "report", report.Name)
		if err := r.ReportTask(ctx, report); err != nil {
			logger.Error(err, "error running report")
			errs = append(errs, err)
			continue
		}
	}

	return errors.Combine(errs...)
}

func (r *ReconcileTask) upload(ctx context.Context) error {
	logger.Info("reconcile upload start")
	meterReports := marketplacev1alpha1.MeterReportList{}
	err := r.K8SClient.List(ctx, &meterReports, client.InNamespace(r.Namespace))
	if err != nil {
		return err
	}

	var errs []error

	meterReportsToRunNames := []string{}
	meterReportsToRun := []*marketplacev1alpha1.MeterReport{}

	// Run reports and upload to data service
	for i := range meterReports.Items {
		report := meterReports.Items[i]
		reason, ok := r.CanRunUploadReportTask(ctx, report)
		if ok {
			logger.Info("not skipping", "report", report.Name)
			meterReportsToRun = append(meterReportsToRun, &report)
			meterReportsToRunNames = append(meterReportsToRunNames, report.Name)
		} else {
			logger.Info("skipping for reason", "report", report.Name, "reason", reason)
			r.setSkipReason(ctx, &report, reason)
		}
	}

	logger.Info("upload: meter reports ready to run", "reports", strings.Join(meterReportsToRunNames, ", "))

	uploadTask, err := r.NewUpload(ctx, r.Config, r.Namespace)
	if err != nil {
		return err
	}

	// Get reports from data service to upload
	for _, report := range meterReportsToRun {
		logger.Info("upload: running report", "report", report.Name)
		if err := uploadTask.RunReport(ctx, report); err != nil {
			details := errors.GetDetails(err)
			logger.Error(err, "error uploading files from data service", details...)
			errs = append(errs, err)
		}
	}

	ok, reason := r.CanRunGenericUpload(ctx)
	if ok {
		if err := uploadTask.RunGeneric(ctx); err != nil {
			details := errors.GetDetails(err)
			logger.Error(err, "error uploading files from data service", details...)
			errs = append(errs, err)
		}
	} else {
		logger.Info("not running generalized upload because of reason", "reason", reason)
	}

	return errors.Combine(errs...)
}

type ReportSkipReason string

const (
	SkipNotReady     ReportSkipReason = "NotReady"
	SkipDisconnected ReportSkipReason = "Disconn"
	SkipNoData       ReportSkipReason = "NoData"
	SkipNoFileID     ReportSkipReason = "NoFileID"
	SkipDone         ReportSkipReason = "Done"
	SkipMaxAttempts  ReportSkipReason = "OutOfRetry"

	NoSkip ReportSkipReason = ""
)

func (r *ReconcileTask) CanRunReportTask(ctx context.Context, report marketplacev1alpha1.MeterReport) bool {
	now := time.Now().UTC()
	if now.Before(report.Spec.EndTime.Time.UTC()) {
		return false
	}

	if report.Status.DataServiceStatus != nil && report.Status.DataServiceStatus.Success() {
		return false
	}

	stat := report.Status.UploadStatus.Get(uploaders.UploaderTargetDataService.Name())
	if stat != nil && stat.Success() {
		return false
	}

	if report.Status.UploadStatus.OneSuccessOf(
		[]string{
			uploaders.UploaderTargetMarketplace.Name(),
			uploaders.UploaderTargetRedHatInsights.Name(),
		},
	) {
		return false
	}

	return true
}

func (r *ReconcileTask) ReportTask(ctx context.Context, report *marketplacev1alpha1.MeterReport) error {
	key := client.ObjectKeyFromObject(report)

	cfg := *r.Config
	cfg.UploaderTargets = uploaders.UploaderTargets{&dataservice.DataService{}}
	task, err := r.NewTask(
		ctx,
		ReportName(key),
		&cfg,
	)

	if err != nil {
		return err
	}

	err = task.Run(ctx)
	if err != nil {
		logger.Error(err, "error running task")
		return err
	}

	return nil
}

//
func (r *ReconcileTask) CanRunGenericUpload(ctx context.Context) (bool, ReportSkipReason) {
	if r.Config.IsDisconnected {
		return false, SkipDisconnected
	}

	return true, NoSkip
}

const maxUploadAttempts = 3

// IsDisconnected defaults to false
func (r *ReconcileTask) CanRunUploadReportTask(ctx context.Context, report marketplacev1alpha1.MeterReport) (ReportSkipReason, bool) {
	if r.Config.IsDisconnected {
		return SkipDisconnected, false
	}

	if report.Status.MetricUploadCount == nil || *report.Status.MetricUploadCount == 0 {
		return SkipNoData, false
	}

	if report.Status.DataServiceStatus == nil || report.Status.DataServiceStatus.ID == "" || !report.Status.DataServiceStatus.Success() {
		return SkipNoFileID, false
	}

	if report.Status.RetryUpload >= maxUploadAttempts {
		return SkipMaxAttempts, false
	}

	return NoSkip, true
}

func (r *ReconcileTask) recordTaskError(ctx context.Context, err error) error {
	var comp *uploaders.ReportJobError

	if errors.As(err, &comp) {

		job := &batchv1.Job{}
		err = r.K8SClient.Get(ctx, types.NamespacedName{Name: os.Getenv("JOB_NAME"), Namespace: os.Getenv("POD_NAMESPACE")}, job)
		if err != nil {
			return err
		}

		jobref, err := reference.GetReference(r.K8SScheme, job)
		if err != nil {
			return err
		}

		r.recorder.Event(jobref, corev1.EventTypeWarning, "ReportJobError", fmt.Sprintf("Report Job Error: %v", comp.Error()))
	}

	return nil
}

func (r *ReconcileTask) setSkipReason(
	ctx context.Context,
	mreport *marketplacev1alpha1.MeterReport,
	skip ReportSkipReason,
) {
	var condition status.Condition
	switch skip {
	case SkipDisconnected:
		condition = marketplacev1alpha1.ReportConditionJobIsDisconnected
	case SkipNoData:
		condition = marketplacev1alpha1.ReportConditionJobHasNoData
	case SkipMaxAttempts:
		condition = marketplacev1alpha1.ReportConditionFailedAttempts
	case NoSkip:
		r.logger.Info("no skip was passed for report", "name", mreport.Name)
		return
	default:
		condition = marketplacev1alpha1.ReportConditionJobSkipped
		condition.Message = fmt.Sprintf("Report skipped because %s", skip)
	}

	if err := updateMeterReportStatus(ctx, r.K8SClient, mreport.Name, mreport.Namespace,
		func(m marketplacev1alpha1.MeterReportStatus) marketplacev1alpha1.MeterReportStatus {
			m.Conditions.SetCondition(condition)
			return m
		}); err != nil {
		logger.Error(err, "failed to update meter report")
	}
}

// idea to use indices to filter the meter report list
// var uploadStatusIndexer, storageStatusIndexer client.FieldIndexer
// const uploadedStatus = "status.conditions.Uploaded"

// _ = uploadStatusIndexer.IndexField(context.TODO(), &marketplacev1alpha1.MeterReport{}, uploadedStatus, func(o client.Object) []string {
// 	var res []string
// 	cond := o.(*marketplacev1alpha1.MeterReport).Status.Conditions.GetCondition(marketplacev1alpha1.ReportConditionTypeUploadStatus)
// 	if cond != nil {
// 		if cond.IsTrue() {
// 			res = append(res, "true")
// 			return res
// 		}

// 		switch cond.Reason {
// 		case marketplacev1alpha1.ReportConditionReasonJobIsDisconnected:
// 			if !r.Config.IsDisconnected {
// 				res = append(res, "false")
// 				return res
// 			}
// 			fallthrough
// 		case marketplacev1alpha1.ReportConditionReasonJobMaxRetries:
// 			fallthrough
// 		case marketplacev1alpha1.ReportConditionReasonJobNoData:
// 			fallthrough
// 		default:
// 			res = append(res, "true")
// 			return res
// 		}
// 	}

// 	res = append(res, "false")
// 	return res
// })

// const storedStatus = "status.conditions.Stored"

// _ = storageStatusIndexer.IndexField(context.TODO(), &marketplacev1alpha1.MeterReport{}, storedStatus, func(o client.Object) []string {
// 	var res []string
// 	cond := o.(*marketplacev1alpha1.MeterReport).Status.Conditions.GetCondition(marketplacev1alpha1.ReportConditionTypeStorageStatus)
// 	if cond != nil && cond.IsTrue() {
// 		res = append(res, "true")
// 	}
// 	res = append(res, "false")
// 	return res
// })
