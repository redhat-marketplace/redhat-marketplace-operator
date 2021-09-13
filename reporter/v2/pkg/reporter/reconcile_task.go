package reporter

import (
	"context"
	"time"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcileTask struct {
	K8SClient client.Client
	Config    *Config
	K8SScheme *runtime.Scheme
	Namespace
}

func (r *ReconcileTask) Run(ctx context.Context) error {
	meterReports := marketplacev1alpha1.MeterReportList{}

	err := r.K8SClient.List(ctx, &meterReports, client.InNamespace(r.Namespace))

	if err != nil {
		return err
	}

	var errs []error

	// Run reports and upload to data service
	for _, report := range meterReports.Items {
		if r.CanRunReportTask(ctx, report) {
			if err := r.ReportTask(ctx, &report); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}

	// Get reports from data service to upload
	if r.CanRunUploadTask(ctx) {
		if err := r.UploadTask(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return nil
}

func (r *ReconcileTask) CanRunReportTask(ctx context.Context, report marketplacev1alpha1.MeterReport) bool {
	now := time.Now().UTC()
	if now.Before(report.Spec.EndTime.Time.UTC()) {
		return false
	}

	return true
}

func (r *ReconcileTask) ReportTask(ctx context.Context, report *marketplacev1alpha1.MeterReport) error {
	key, err := client.ObjectKeyFromObject(report)
	if err != nil {
		return err
	}

	cfg := *r.Config
	cfg.UploaderTargets = UploaderTargets{&DataServiceUploader{}}
	task, err := NewTask(
		ctx,
		ReportName(key),
		&cfg,
	)

	if err != nil {
		return err
	}

	err = task.Run()
	if err != nil {
		logger.Error(err, "error running task")
		return err
	}

	return nil
}

func (r *ReconcileTask) CanRunUploadTask(ctx context.Context) bool {
	return true
}

func (r *ReconcileTask) UploadTask(ctx context.Context) error {
	cfg := *r.Config

	uploadTask, err := NewUploadTask(
		ctx,
		&cfg,
	)

	if err != nil {
		logger.Error(err, "error running task")
		return err
	}

	err = uploadTask.Run()
	if err != nil {
		logger.Error(err, "error running task")
		return err
	}

	return nil
}
