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
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcileTask struct {
	K8SClient client.Client
	Config    *Config
	K8SScheme *runtime.Scheme
	Namespace
	recorder record.EventRecorder
}

func (r *ReconcileTask) Run(ctx context.Context) error {

	err := r.run(ctx)

	if err != nil {
		terr := r.recordTaskError(ctx, err)
		if terr != nil {
			logger.Error(terr, "error recording task error to events")
		}

		return err
	}

	return nil
}

func (r *ReconcileTask) run(ctx context.Context) error {
	logger.Info("reconcile run start")
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

	logger.Info("meter reports ready to run", "reports", strings.Join(meterReportsToRunNames, ", "))

	for _, report := range meterReportsToRun {
		if err := r.ReportTask(ctx, report); err != nil {
			logger.Error(err, "error running report")
			errs = append(errs, err)
			continue
		}
	}

	// Get reports from data service to upload
	if r.CanRunUploadTask(ctx) {
		if err := r.UploadTask(ctx); err != nil {
			logger.Error(err, "error uploading files from data service")
			return err
			// errs = append(errs, err)
		}
	}

	return nil
}

func (r *ReconcileTask) CanRunReportTask(ctx context.Context, report marketplacev1alpha1.MeterReport) bool {
	now := time.Now().UTC()
	if now.Before(report.Spec.EndTime.Time.UTC()) {
		return false
	}

	uploadStatus := report.Status.Conditions.GetCondition(marketplacev1alpha1.ReportConditionTypeUploadStatus)

	if uploadStatus != nil && uploadStatus.IsTrue() {
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

func (r *ReconcileTask) recordTaskError(ctx context.Context, err error) error {

	var comp *ReportJobError

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
