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

package meterreport

import (
	"context"
	"reflect"
	"time"

	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_meterreport")

// Add creates a new MeterReport Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
	cfg *config.OperatorConfig,
) error {
	return add(mgr, newReconciler(mgr, ccprovider, cfg))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
	cfg *config.OperatorConfig,
) reconcile.Reconciler {
	return &ReconcileMeterReport{
		client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		ccprovider: ccprovider,
		patcher:    patch.RHMDefaultPatcher,
		cfg:        cfg,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("meterreport-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MeterReport
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MeterReport{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterReport{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner MeterReport
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterReport{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMeterReport implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMeterReport{}

// ReconcileMeterReport reconciles a MeterReport object
type ReconcileMeterReport struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	cfg        *config.OperatorConfig
	ccprovider ClientCommandRunnerProvider
	patcher    patch.Patcher
}

// Reconcile reads that state of the cluster for a MeterReport object and makes changes based on the state read
// and what is in the MeterReport.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMeterReport) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterReport")

	cc := r.ccprovider.NewCommandRunner(r.client, r.scheme, reqLogger)

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
		instance.Status.Conditions = &conds
	}

	job := &batchv1.Job{}

	c := manifests.NewOperatorConfig(r.cfg)
	factory := manifests.NewFactory(instance.Namespace, c)

	reqLogger.Info("config",
		"config", r.cfg.Image,
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
				UpdateStatusCondition(instance, instance.Status.Conditions, marketplacev1alpha1.ReportConditionJobWaiting),
				OnAny(RequeueAfterResponse(timeToWait)),
			),
		)
		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get create job.")
		}

		return result.Return()

	}

	result, _ := cc.Do(
		context.TODO(),
		HandleResult(
			manifests.CreateIfNotExistsFactoryItem(
				job,
				func() (runtime.Object, error) {
					return factory.ReporterJob(instance)
				}, CreateWithAddOwner(instance),
			),
			OnAny(UpdateStatusCondition(instance, instance.Status.Conditions, marketplacev1alpha1.ReportConditionJobSubmitted)),
		),
	)

	if !result.Is(Continue) {
		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get create job.")
		}

		return result.Return()
	}

	jr := &common.JobReference{}
	jr.SetFromJob(job)

	// if job is not done, then update status and continue
	if !jr.IsDone() {
		if !reflect.DeepEqual(instance.Status.AssociatedJob, jr) {
			instance.Status.AssociatedJob = jr

			reqLogger.Info("Updating MeterReport status associatedJob")
			if result, _ := cc.Do(context.TODO(), UpdateAction(instance, UpdateStatusOnly(true))); !result.Is(Continue) {
				if result.Is(Error) {
					reqLogger.Error(result.GetError(), "Failed to get update status.")
					return reconcile.Result{Requeue: true}, result
				}

				return result.Return()
			}
		}

		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	instance.Status.AssociatedJob = jr

	// if report failed
	switch {
	case jr.IsFailed():
		reqLogger.Info("job failed")
		result, _ = cc.Do(context.TODO(),
			DeleteAction(job, DeleteWithDeleteOptions(client.PropagationPolicy(metav1.DeletePropagationBackground))),
			HandleResult(
				UpdateStatusCondition(
					instance,
					instance.Status.Conditions,
					marketplacev1alpha1.ReportConditionJobErrored),
				OnAny(RequeueAfterResponse(1*time.Hour)),
			),
		)
	case jr.IsSuccessful():
		reqLogger.Info("job is complete")
		result, _ = cc.Do(context.TODO(),
			UpdateStatusCondition(instance, instance.Status.Conditions, marketplacev1alpha1.ReportConditionJobFinished),
		)

	}

	if result != nil && !result.Is(Continue) {
		return result.Return()
	}

	reqLogger.Info("reconcile finished")
	return reconcile.Result{}, nil
}
