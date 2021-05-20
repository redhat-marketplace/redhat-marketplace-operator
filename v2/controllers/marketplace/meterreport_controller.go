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
	"math/rand"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
)

// blank assignment to verify that ReconcileMeterReport implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterReportReconciler{}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch;extensions,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch;extensions,namespace=system,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterreports;meterreports/status;meterreports/finalizers,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=meterreports;meterreports/status;meterreports/finalizers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:urls=/api/v1/query;/api/v1/query_range,verbs=get;create

// MeterReportReconciler reconciles a MeterReport object
type MeterReportReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	CC      ClientCommandRunner
	patcher patch.Patcher
	cfg     *config.OperatorConfig
	factory *manifests.Factory
}

func (r *MeterReportReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
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

func (r *MeterReportReconciler) InjectFactory(f *manifests.Factory) error {
	r.factory = f
	return nil
}

func (m *MeterReportReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	m.cfg = cfg
	return nil
}

func (r *MeterReportReconciler) SetupWithManager(mgr manager.Manager) error {
	namespacePredicate := predicates.NamespacePredicate(r.cfg.DeployedNamespace)

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(namespacePredicate).
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

const rerunTime = 8 * 24 * time.Hour

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

	endTime := instance.Spec.EndTime.Time
	now := time.Now()

	reqLogger.Info("time", "now", now, "endTime", endTime)

	if now.Before(endTime) {
		wait := waitTime(now, endTime, rand.Intn(59))
		reqLogger.Info("report was schedule before it was ready to run", "add", wait)
		result, _ := cc.Do(
			context.TODO(),
			HandleResult(
				UpdateStatusCondition(instance, &instance.Status.Conditions, marketplacev1alpha1.ReportConditionJobWaiting),
				OnAny(RequeueAfterResponse(wait)),
			),
		)
		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get create job.")
		}

		return result.Return()
	}

	// getting job
	result, _ := cc.Do(context.TODO(), GetAction(types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}, job))

	// We'll rerun the jobs of the last 7 days in case we push a fix
	lastVersion, hasAnnotation := instance.GetAnnotations()["marketplace.redhat.com/version"]
	rerunDate := time.Now().Add(-rerunTime)

	if !hasAnnotation || (hasAnnotation && lastVersion != version.Version) {
		reqLogger.Info("new version detected, updating version annotation")

		annotations := instance.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		annotations["marketplace.redhat.com/version"] = version.Version
		instance.SetAnnotations(annotations)
		instance.Status.AssociatedJob = nil

		if instance.Status.AssociatedJob != nil && instance.Spec.StartTime.After(rerunDate) {
			reqLogger.Info("job is within requeue time, deleting old job")
			result, _ = cc.Do(context.TODO(),
				HandleResult(
					GetAction(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, job),
					OnContinue(DeleteAction(job, DeleteWithDeleteOptions(client.PropagationPolicy(metav1.DeletePropagationBackground))))),
			)

			if result.Is(Error) {
				reqLogger.Error(result.Err, "error updating")
				return reconcile.Result{}, result.Err
			}
		}

		result, _ = cc.Do(context.TODO(), UpdateAction(instance))

		if result.Is(Error) {
			reqLogger.Error(result.Err, "error updating")
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.Info("new version detected, updating version annotation")
		return reconcile.Result{Requeue: true}, nil
	}

	if instance.Status.AssociatedJob != nil &&
		instance.Status.AssociatedJob.IsSuccessful() &&
		!result.Is(NotFound) {
		reqLogger.Info("reconcile finished, job successful")
		return reconcile.Result{}, nil
	}

	// Create associated job
	if instance.Status.AssociatedJob == nil {
		result, _ := cc.Do(context.TODO(),
			HandleResult(
				manifests.CreateIfNotExistsFactoryItem(
					job,
					func() (runtime.Object, error) {
						return r.factory.ReporterJob(instance, r.cfg.ReportController.RetryLimit)
					}, CreateWithAddController(instance),
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
	result, _ = cc.Do(
		context.TODO(),
		HandleResult(
			GetAction(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, job),
			OnNotFound(Call(func() (ClientAction, error) {
				reqLogger.Info("job not found")

				if instance.Status.AssociatedJob != nil {
					instance.Status.AssociatedJob = nil
					reqLogger.Info("Updating MeterReport status associatedJob to nil")
					return UpdateAction(instance, UpdateStatusOnly(true)), nil
				}

				return nil, nil
			})),
		),
	)

	if !result.Is(Continue) {
		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to on resolving job.")
		}
		return result.Return()
	}

	if job == nil {
		err := errors.New("job cannot be nil")
		reqLogger.Error(err, "Failed to on resolving job.")
		return reconcile.Result{}, err
	}

	jr := &common.JobReference{}
	jr.SetFromJob(job)

	reqLogger.Info(
		"reviewing job",
		"jr", jr,
		"done", jr.IsDone(),
		"failed", jr.IsFailed(),
		"success", jr.IsSuccessful(),
	)

	// if report failed
	switch {
	case jr.IsFailed():
		completionTimeDiff := now.Sub(jr.StartTime.Time)

		switch {
		case completionTimeDiff >= r.cfg.ReportController.RetryTime:
			reqLogger.Info("job failed, deleteing and requeuing", "retryTime",
				r.cfg.ReportController.RetryTime,
				"diff", completionTimeDiff)
			instance.Status.AssociatedJob = nil
			result, _ = cc.Do(context.TODO(),
				HandleResult(
					GetAction(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, job),
					OnContinue(DeleteAction(job, DeleteWithDeleteOptions(client.PropagationPolicy(metav1.DeletePropagationBackground))))),
				UpdateAction(instance),
			)
		default:
			reqLogger.Info("job failed, requeuing in an hour", "time", completionTimeDiff)
			instance.Status.AssociatedJob = jr
			result, _ = cc.Do(context.TODO(),
				UpdateStatusCondition(
					instance,
					&instance.Status.Conditions,
					marketplacev1alpha1.ReportConditionJobErrored),
				UpdateAction(instance),
				RequeueAfterResponse(time.Hour),
			)
		}
	case jr.IsSuccessful():
		reqLogger.Info("job is complete")
		instance.Status.AssociatedJob = jr
		result, _ = cc.Do(context.TODO(),
			UpdateStatusCondition(instance, &instance.Status.Conditions, marketplacev1alpha1.ReportConditionJobFinished),
		)
	default:
		reqLogger.Info("job not done", "jr", jr)
		if instance.Status.AssociatedJob == nil ||
			!reflect.DeepEqual(instance.Status.AssociatedJob, jr) {
			instance.Status.AssociatedJob = jr

			reqLogger.Info("Updating MeterReport status associatedJob")
			result, _ = cc.Do(context.TODO(),
				UpdateAction(instance, UpdateStatusOnly(true)),
			)
		}
	}

	if result != nil && !result.Is(Continue) {
		return result.Return()
	}

	reqLogger.Info("reconcile finished")
	return reconcile.Result{}, nil
}

func waitTime(now time.Time, timeToExecute time.Time, addRandom int) time.Duration {
	waitTime := timeToExecute.Sub(now)
	waitTime = waitTime + time.Minute*time.Duration(addRandom)

	if waitTime < 0 {
		return time.Duration(0)
	}

	return waitTime
}
