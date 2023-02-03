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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// blank assignment to verify that ReconcileMeterReport implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterReportReconciler{}

// +kubebuilder:rbac:groups=batch;extensions,namespace=system,resources=jobs,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=meterreports;meterreports/status;meterreports/finalizers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:urls=/api/v1/query;/api/v1/query_range;/api/v1/targets,verbs=get;create

// MeterReportReconciler reconciles a MeterReport object
type MeterReportReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	cfg     *config.OperatorConfig
	factory *manifests.Factory
}

func (r *MeterReportReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
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
		Complete(r)
}

// Reconcile reads that state of the cluster for a MeterReport object and makes changes based on the state read
// and what is in the MeterReport.Spec

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *MeterReportReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterReport")

	// Fetch the MeterReport instance
	instance := &marketplacev1alpha1.MeterReport{}
	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("MeterReport resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get MeterReport.")
		return reconcile.Result{}, err
	}

	if instance.Status.Conditions == nil {
		conds := status.NewConditions(marketplacev1alpha1.ReportConditionJobNotStarted)
		instance.Status.Conditions = conds
	}

	// getting && deleting job; new process uses a single cronjob
	if instance.Status.AssociatedJob != nil {
		job := &batchv1.Job{}
		job.Name = instance.Status.AssociatedJob.Name
		job.Namespace = instance.Status.AssociatedJob.Namespace

		if err := r.Client.Delete(context.TODO(), job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to delete job.")
			return reconcile.Result{}, err
		}

		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
				return err
			}
			instance.Status.AssociatedJob = nil
			return r.Client.Update(context.TODO(), instance)
		}); err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "error updating MeterReport")
			return reconcile.Result{Requeue: true}, err
		}
	}
	// done getting and delete old job

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
