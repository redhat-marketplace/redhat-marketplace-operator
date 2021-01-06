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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/inject"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// blank assignment to verify that ReconcileMeterReport implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterReportReconciler{}

// MeterReportReconciler reconciles a MeterReport object
type MeterReportReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	CC      ClientCommandRunner
	patcher patch.Patcher
	cfg     config.OperatorConfig
	factory manifests.Factory
}

func (r *MeterReportReconciler) Inject(injector *inject.Injector) inject.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *MeterReportReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.Log.Info("command runner")
	r.CC = ccp
	return nil
}

func (r *MeterReportReconciler) InjectPatch(p patch.Patcher) error {
	r.patcher = p
	return nil
}

func (r *MeterReportReconciler) InjectFactory(f manifests.Factory) error {
	r.factory = f
	return nil
}

func (r *MeterReportReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MeterReport{}).
		Watches(&source.Kind{Type: &marketplacev1alpha1.MeterReport{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &marketplacev1alpha1.MeterReport{},
		}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &marketplacev1alpha1.MeterReport{},
		}).
		Complete(r)
}

// Reconcile reads that state of the cluster for a MeterReport object and makes changes based on the state read
// and what is in the MeterReport.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *MeterReportReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterReport")

	cc := r.CC

	// Fetch the MeterReport instance
	instance := &marketplacev1alpha1.MeterReport{}

	if result, _ := cc.Do(context.TODO(), GetAction(request.NamespacedName, instance)); !result.Is(Continue) {
		if result.Is(NotFound) {
			reqLogger.Info("MeterReport resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}

		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get MeterReport.")
		}

		return result.Return()
	}

	if instance.Status.Conditions == nil {
		conds := status.NewConditions(marketplacev1alpha1.ReportConditionJobNotStarted)
		instance.Status.Conditions = conds
	}

	job := &batchv1.Job{}

	reqLogger.Info("config",
		"config", r.cfg.RelatedImages,
		"envvar", utils.Getenv("RELATED_IMAGE_REPORTER", ""))

	endTime := instance.Spec.EndTime.UTC()
	now := metav1.Now().UTC()

	reqLogger.Info("time", "now", now, "endTime", endTime)

	if now.UTC().Before(endTime.UTC()) {
		waitTime := now.Add(endTime.Sub(now))
		waitTime.Add(time.Minute * 5)
		timeToWait := waitTime.Sub(now)
		reqLogger.Info("report was schedule before it was ready to run", "add", timeToWait)
		result, _ := cc.Do(
			context.TODO(),
			HandleResult(
				UpdateStatusCondition(instance, &instance.Status.Conditions, marketplacev1alpha1.ReportConditionJobWaiting),
				OnAny(RequeueAfterResponse(timeToWait)),
			),
		)
		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get create job.")
		}

		return result.Return()
	}

	// Create associated job
	if instance.Status.AssociatedJob == nil {
		result, _ := cc.Do(context.TODO(),
			HandleResult(
				manifests.CreateIfNotExistsFactoryItem(
					job,
					func() (runtime.Object, error) {
						return r.factory.ReporterJob(instance)
					}, CreateWithAddOwner(instance),
				),
				OnRequeue(UpdateStatusCondition(instance, &instance.Status.Conditions, marketplacev1alpha1.ReportConditionJobSubmitted)),
			),
		)

		if !result.Is(Continue) {
			if result.Is(Error) {
				reqLogger.Error(result.GetError(), "Failed to on resolving job.")
			}
			return result.Return()
		}
	}

	// Update associated job
	result, _ := cc.Do(
		context.TODO(),
		HandleResult(
			GetAction(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, job),
			OnContinue(Call(func() (ClientAction, error) {
				jr := &common.JobReference{}
				jr.SetFromJob(job)
				instance.Status.AssociatedJob = jr

				if !reflect.DeepEqual(instance.Status.AssociatedJob, jr) {
					instance.Status.AssociatedJob = jr

					reqLogger.Info("Updating MeterReport status associatedJob")
					return UpdateAction(instance, UpdateStatusOnly(true)), nil
				}

				return nil, nil
			}))),
	)

	if !result.Is(Continue) {
		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to on resolving job.")
		}
		return result.Return()
	}

	jr := instance.Status.AssociatedJob
	reqLogger.Info("reviewing job", "jr", jr,
		"active", jr.Active,
		"failed", jr.Failed,
		"backoff", jr.BackoffLimit,
		"success", jr.Succeeded,
	)

	// if report failed
	switch {
	case !instance.Status.AssociatedJob.IsDone():
		reqLogger.Info("requeueing job not done", "jr", jr)
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case instance.Status.AssociatedJob.CompletionTime != nil &&
		!instance.Status.AssociatedJob.IsSuccessful():
		completionTime := instance.Status.AssociatedJob.CompletionTime.UTC()
		completionTimeDiffHours := now.Sub(completionTime).Hours()

		switch {
		case instance.Status.AssociatedJob.IsFailed() &&
			completionTimeDiffHours >= 4:
			reqLogger.Info("job failed, deleteing and requeuing for an hour")
			instance.Status.AssociatedJob = nil
			result, _ = cc.Do(context.TODO(),
				HandleResult(
					GetAction(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, job),
					OnContinue(DeleteAction(job, DeleteWithDeleteOptions(client.PropagationPolicy(metav1.DeletePropagationBackground))))),
				UpdateAction(instance),
			)
		case instance.Status.AssociatedJob.IsFailed() &&
			completionTimeDiffHours < 4:
			requeueTime := (time.Hour * 4) - now.Sub(completionTime)
			reqLogger.Info("job failed, deleteing and requeuing for in 4 hour", "requeueTime", requeueTime)
			result, _ = cc.Do(context.TODO(),
				UpdateStatusCondition(
					instance,
					&instance.Status.Conditions,
					marketplacev1alpha1.ReportConditionJobErrored),
				RequeueAfterResponse(requeueTime),
			)
		}
	case instance.Status.AssociatedJob.IsSuccessful():
		reqLogger.Info("job is complete")
		result, _ = cc.Do(context.TODO(),
			HandleResult(
				GetAction(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, job),
				OnContinue(DeleteAction(job, DeleteWithDeleteOptions(client.PropagationPolicy(metav1.DeletePropagationBackground))))),
			UpdateStatusCondition(instance, &instance.Status.Conditions, marketplacev1alpha1.ReportConditionJobFinished),
		)
	}

	if result != nil && !result.Is(Continue) {
		return result.Return()
	}

	reqLogger.Info("reconcile finished")
	return reconcile.Result{}, nil
}
