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

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	MeteredResourceAnnotationKey = "marketplace.redhat.com/meteredUIDs"
)

// blank assignment to verify that ReconcileMeterDefinition implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterDefinitionReconciler{}

// MeterDefinitionReconciler reconciles a MeterDefinition object
type MeterDefinitionReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	cfg    *config.OperatorConfig

	prometheusAPIBuilder *prometheus.PrometheusAPIBuilder
}

func (r *MeterDefinitionReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (m *MeterDefinitionReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	m.cfg = cfg
	return nil
}

func (m *MeterDefinitionReconciler) InjectPrometheusAPIBuilder(b *prometheus.PrometheusAPIBuilder) error {
	m.prometheusAPIBuilder = b
	return nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *MeterDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a new controller
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.MeterDefinition{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewMaxOfRateLimiter(
				workqueue.DefaultControllerRateLimiter(),
				workqueue.NewItemExponentialFailureRateLimiter(time.Second, 15*time.Minute),
			),
		}).
		Complete(r)
}

// Prometheus Client
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions/status,verbs=get;list;update;patch
// +kubebuilder:rbac:groups="",namespace=system,resources=serviceaccounts/token,verbs=get;list;create;update

// Reconcile reads that state of the cluster for a MeterDefinition object and makes changes based on the state read
// and what is in the MeterDefinition.Spec
func (r *MeterDefinitionReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterDefinition")

	// Fetch the MeterDefinition instance
	instance := &v1beta1.MeterDefinition{}
	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Info("MeterDefinition resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get MeterDefinition.")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Found instance", "instance", instance.Name)

	var requeue bool

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}

		var update bool

		// Set the Status Condition of signature signing
		if instance.IsSigned() {
			// Check the signature
			if err := instance.ValidateSignature(); err != nil {
				// This could occur after the admission webhook if a signed v1alpha1 is converted to v1beta1.
				// However, signing not introduced until v1beta1, so this is unlikely.
				update = update || instance.Status.Conditions.SetCondition(common.MeterDefConditionSignatureVerificationFailed)
			} else {
				update = update || instance.Status.Conditions.SetCondition(common.MeterDefConditionSignatureVerified)
			}
		} else {
			// Unsigned, Unverified
			update = update || instance.Status.Conditions.SetCondition(common.MeterDefConditionSignatureUnverified)
		}

		switch {
		case instance.Status.Conditions.IsUnknownFor(common.MeterDefConditionTypeHasResult):
			fallthrough
		case len(instance.Status.WorkloadResources) == 0:
			update = update || instance.Status.Conditions.SetCondition(common.MeterDefConditionNoResults)
		case len(instance.Status.WorkloadResources) > 0:
			update = update || instance.Status.Conditions.SetCondition(common.MeterDefConditionHasResults)
		}

		if update {
			return r.Client.Status().Update(context.TODO(), instance)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// meterbase & userWorkloadMonitoring is required, use the meterbase status to confirm readiness
	meterbase := &v1alpha1.MeterBase{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.METERBASE_NAME,
		Namespace: r.cfg.DeployedNamespace,
	}, meterbase); k8serrors.IsNotFound(err) {
		// no MeterBase, requeue
		reqLogger.Info("meterbase not found, unable to generate meterdefinition query preview")
		return reconcile.Result{RequeueAfter: r.cfg.ControllerValues.MeterDefControllerRequeueRate}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	userWorkloadMonitoringEnabled := meterbase.Status.Conditions.IsTrueFor(v1alpha1.ConditionUserWorkloadMonitoringEnabled)

	if !userWorkloadMonitoringEnabled {
		reqLogger.Info("user workload monitoring is not enabled, unable to generate meterdefinition query preview")
		return reconcile.Result{RequeueAfter: r.cfg.ControllerValues.MeterDefControllerRequeueRate}, nil
	}

	// Verify reporting
	isReporting, err := r.verifyReporting(instance, userWorkloadMonitoringEnabled, reqLogger)

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
			return err
		}

		var update bool

		if isReporting {
			update = update || instance.Status.Conditions.SetCondition(common.MeterDefConditionReporting)
		} else {
			update = update || instance.Status.Conditions.SetCondition(common.MeterDefConditionNotReporting)
		}
		if err != nil {
			condition := v1beta1.VerifyReportingErrorCondition
			condition.Message = err.Error()
			update = update || instance.Status.Conditions.SetCondition(condition)
			requeue = true
		} else {
			update = update || instance.Status.Conditions.RemoveCondition(v1beta1.MeterDefVerifyReportingSetupError)
		}

		if update {
			return r.Client.Status().Update(context.TODO(), instance)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	if requeue {
		return reconcile.Result{RequeueAfter: r.cfg.ControllerValues.MeterDefControllerRequeueRate}, nil
	}

	// Generate preview
	queryPreviewResult, err := r.queryPreview(instance, request, reqLogger, userWorkloadMonitoringEnabled)

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var update bool

		if err != nil {
			condition := v1beta1.PreviewErrorCondition
			condition.Message = err.Error()
			update = update || instance.Status.Conditions.SetCondition(condition)
			requeue = true
		}

		if err == nil && len(queryPreviewResult) != 0 {
			update = update || instance.Status.Conditions.RemoveCondition(v1beta1.MeterDefQueryPreviewSetupError)

			if !reflect.DeepEqual(queryPreviewResult, instance.Status.Results) {
				instance.Status.Results = queryPreviewResult
				reqLogger.Info("output", "Status.Results", instance.Status.Results)
				update = update || true
			}
		}

		if update {
			return r.Client.Status().Update(context.TODO(), instance)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	if requeue {
		reqLogger.Info("error happened while trying to generate preview, requeue faster")
		return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	reqLogger.Info("meterdef_preview", "requeue rate", r.cfg.ControllerValues.MeterDefControllerRequeueRate)
	reqLogger.Info("finished reconciling")

	return reconcile.Result{RequeueAfter: r.cfg.ControllerValues.MeterDefControllerRequeueRate}, nil
}

func (r *MeterDefinitionReconciler) queryPreview(instance *v1beta1.MeterDefinition, request reconcile.Request, reqLogger logr.Logger, userWorkloadMonitoringEnabled bool) ([]common.Result, error) {
	var queryPreviewResult []common.Result

	prometheusAPI, err := r.prometheusAPIBuilder.Get(r.prometheusAPIBuilder.GetAPITypeFromFlag(userWorkloadMonitoringEnabled))

	if err != nil {
		return queryPreviewResult, err
	}

	reqLogger.Info("generatring meterdef preview")
	return generateQueryPreview(instance, prometheusAPI, reqLogger)
}

func returnQueryRange(duration time.Duration) (startTime time.Time, endTime time.Time) {
	endTime = time.Now().UTC().Truncate(time.Hour)
	startTime = endTime.Add(-duration)
	return
}

func generateQueryPreview(instance *v1beta1.MeterDefinition, prometheusAPI *prometheus.PrometheusAPI, reqLogger logr.Logger) (queryPreviewResultArray []common.Result, returnErr error) {
	var queryPreviewResult *common.Result
	labels := instance.ToPrometheusLabels()

	for _, meterWorkload := range labels {
		var val model.Value
		startTime, endTime := returnQueryRange(meterWorkload.MetricPeriod.Duration)
		query := prometheus.PromQueryFromLabels(meterWorkload, startTime, endTime)

		q, err := query.Print()
		if err != nil {
			return nil, err
		}

		reqLogger.Info("meterdef preview query", "query", q)

		var warnings v1.Warnings
		err = utils.Retry(func() error {
			var err error
			val, warnings, err = prometheusAPI.ReportQuery(query)
			if err != nil {
				return errors.Wrap(err, "error with query")
			}

			reqLogger.Info("query preview", "model value: ", val)

			return nil
		}, *ptr.Int(2))

		if warnings != nil {
			reqLogger.Info("warnings", "warnings", warnings)
		}

		if err != nil {
			reqLogger.Error(err, "prometheus.QueryRange()")
			returnErr = errors.Wrap(err, "error with query")
			return nil, returnErr
		}

		queryPreviewResult = &common.Result{
			MetricName: meterWorkload.Metric,
			Query:      q,
			Values:     []common.ResultValues{},
		}

		if val.Type() == model.ValMatrix {
			matrix := val.(model.Matrix)
			for _, m := range matrix {
				for _, pair := range m.Values {
					queryPreviewResult.Values = append(queryPreviewResult.Values, common.ResultValues{
						Timestamp: pair.Timestamp.Unix(),
						Value:     pair.Value.String(),
					})
				}
			}
		}

		queryPreviewResultArray = append(queryPreviewResultArray, *queryPreviewResult)
	}

	return queryPreviewResultArray, nil
}

// Is Prometheus reporting on the MeterDefinition
// Check MeterDefinition presence in api/v1/label/meter_def_name/values
func (r *MeterDefinitionReconciler) verifyReporting(instance *v1beta1.MeterDefinition, userWorkloadMonitoringEnabled bool, reqLogger logr.Logger) (bool, error) {
	reqLogger.Info("apibuilder", "api", r.prometheusAPIBuilder)
	prometheusAPI, err := r.prometheusAPIBuilder.Get(r.prometheusAPIBuilder.GetAPITypeFromFlag(userWorkloadMonitoringEnabled))

	if err != nil {
		return false, err
	}

	reqLogger.Info("getting meter_def_name labelvalues from prometheus")

	mdefLabelValue := model.LabelValue(instance.Name)

	// prometheus/client_golang v1.10.0 modified the api to add matches
	// https://github.com/prometheus/client_golang/pull/828
	// controller-util currently requires client_golang v1.11.0
	// use of matches in our use case results in an error "bad_data: 1:4: parse error: unexpected <op:->"

	//matches := []string{string(mdefLabelValue)}
	matches := []string{}

	labelValues, warnings, err := prometheusAPI.MeterDefLabelValues(matches)

	if warnings != nil {
		reqLogger.Info("warnings", "warnings", warnings)
	}

	if err != nil {
		reqLogger.Error(err, "prometheus.LabelValues()")
		returnErr := errors.Wrap(err, "error with query")
		return false, returnErr
	}

	for _, labelValue := range labelValues {
		if labelValue == mdefLabelValue {
			return true, nil
		}
	}

	return false, nil
}
