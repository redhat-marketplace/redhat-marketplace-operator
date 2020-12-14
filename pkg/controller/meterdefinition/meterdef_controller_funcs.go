package meterdefinition

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	"github.com/operator-framework/operator-sdk/pkg/status"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	v1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func test(){
	
}

func (r *ReconcileMeterDefinition) QueryPreview(cc ClientCommandRunner, instance *v1alpha1.MeterDefinition, request reconcile.Request, reqLogger logr.Logger) (update bool) {
	var queryPreviewResult []v1alpha1.Result

	service, err := queryForPrometheusService(context.TODO(), cc, request)
	update = updateOrClearErrorConditions(err, v1alpha1.PrometheusReconcileError, instance, request, reqLogger)
	if update {
		return update
	}

	certConfigMap, err := getCertConfigMap(context.TODO(), cc, request)
	update = updateOrClearErrorConditions(err, v1alpha1.GetCertConfigMapReconcileError, instance, request, reqLogger)
	if update {
		return update
	}

	saClient := NewServiceAccountClient(instance.Namespace, r.kubernetesInterface)

	authToken, err := saClient.NewServiceAccountToken(utils.OPERATOR_SERVICE_ACCOUNT, utils.PrometheusAudience, 3600, reqLogger)
	update = updateOrClearErrorConditions(err, v1alpha1.ProvideAuthTokenReconcileError, instance, request, reqLogger)
	if update {
		return update
	}

	if certConfigMap != nil && authToken != "" && service != nil {
		cert, err := parseCertificateFromConfigMap(*certConfigMap)
		update = updateOrClearErrorConditions(err, v1alpha1.ParseCertFromConfigMapError, instance, request, reqLogger)
		if update {
			return update
		}

		prometheusAPI,err := NewPromAPI(service,&cert,authToken)
		update = updateOrClearErrorConditions(err, v1alpha1.NewPromAPIError, instance, request, reqLogger)
		if update {
			return update
		}

		reqLogger.Info("generatring meterdef preview")
		queryPreviewResult, err = generateQueryPreview(instance,prometheusAPI ,reqLogger)
		update = updateOrClearErrorConditions(err, v1alpha1.QueryPreviewGenerationError, instance, request, reqLogger)
		if update {
			return update
		}
	}

	if !reflect.DeepEqual(queryPreviewResult, instance.Status.Results) {
		instance.Status.Results = queryPreviewResult
		reqLogger.Info("output", "Status.Results", instance.Status.Results)
		return true
	}

	return false
}

func updateOrClearErrorConditions(err error, conditionReason status.ConditionReason, instance *v1alpha1.MeterDefinition, request reconcile.Request, reqLogger logr.Logger) bool {

	if err != nil {
		reqLogger.Info("Updating status with error")

		update := instance.Status.Conditions.SetCondition(status.Condition{
			Message: err.Error(),
			Reason:  status.ConditionReason(conditionReason),
		})

		return update
	}

	update := clearErrorCondition(conditionReason, instance, reqLogger, request)
	if update {
		return update
	}

	return false
}

//TODO: not being used
func (r *ReconcileMeterDefinition) UpdateStatus(update bool, instance *v1alpha1.MeterDefinition, request reconcile.Request, reqLogger logr.Logger) *ExecResult {

	if update {
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			if k8serrors.IsConflict(err) {
				return r.handleUpdateConflictForStatusOnly(instance, request, reqLogger)
			}

			return &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}
		}

		return &ExecResult{
			ReconcileResult: reconcile.Result{Requeue: true},
			Err:             nil,
		}
	}

	return &ExecResult{
		Status: Continue,
	}

}

//TODO: not being used
func (r *ReconcileMeterDefinition) handleUpdateConflictForStatusOnly(instance *v1alpha1.MeterDefinition, request reconcile.Request, reqLogger logr.Logger) *ExecResult {
	reqLogger.Info("conflict err")

	latestMeterdef := v1alpha1.MeterDefinition{}
	r.client.Get(context.TODO(), request.NamespacedName, &latestMeterdef)

	latestMeterdef.Status = instance.Status
	err := r.client.Status().Update(context.TODO(), &latestMeterdef)
	if err != nil {
		reqLogger.Error(err, "error updating with resource version port")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	return &ExecResult{
		ReconcileResult: reconcile.Result{Requeue: true},
		Err:             err,
	}
}

/*
	had to write a one-off here. Conditions.RemoveCondition() takes a ConditionType which we're not setting
*/
func clearErrorCondition(conditionReason status.ConditionReason, instance *v1alpha1.MeterDefinition, reqLogger logr.Logger, request reconcile.Request) (update bool) {
	for index, condition := range instance.Status.Conditions {
		if condition.Reason == conditionReason {
			reqLogger.Info("clearing condition", "condition", conditionReason)
			instance.Status.Conditions = append(instance.Status.Conditions[:index], instance.Status.Conditions[index+1:]...)
			return true
		}
	}

	return false
}

func queryForPrometheusService(
	ctx context.Context,
	cc ClientCommandRunner,
	req reconcile.Request,
) (*corev1.Service, error) {
	service := &corev1.Service{}

	name := types.NamespacedName{
		Name:      utils.PROMETHEUS_METERBASE_NAME,
		Namespace: req.Namespace,
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		return nil, errors.Wrap(result, "failed to get prometheus service")
	}

	log.Info("retrieved prometheus service")
	return service, nil
}

func getCertConfigMap(ctx context.Context, cc ClientCommandRunner, req reconcile.Request) (*corev1.ConfigMap, error) {
	certConfigMap := &corev1.ConfigMap{}

	name := types.NamespacedName{
		Name:      utils.OPERATOR_CERTS_CA_BUNDLE_NAME,
		Namespace: req.Namespace,
	}

	if result, _ := cc.Do(context.TODO(), GetAction(name, certConfigMap)); !result.Is(Continue) {
		return nil, errors.Wrap(result.GetError(), "Failed to retrieve operator-certs-ca-bundle.")
	}

	log.Info("retrieved configmap")
	return certConfigMap, nil
}

func parseCertificateFromConfigMap(certConfigMap corev1.ConfigMap) (cert []byte, returnErr error) {
	log.Info("extracting cert from config map")

	out, ok := certConfigMap.Data["service-ca.crt"]

	if !ok {
		returnErr = errors.New("Error retrieving cert from config map")
		return nil, returnErr
	}

	cert = []byte(out)
	return cert, nil
}

func generateQueryPreview(instance *v1alpha1.MeterDefinition, prometheusAPI *PrometheusAPI, reqLogger logr.Logger) (queryPreviewResultArray []v1alpha1.Result, returnErr error) {
	loc, _ := time.LoadLocation("UTC")
	var queryPreviewResult *v1alpha1.Result

	for _, workload := range instance.Spec.Workloads {
		var val model.Value
		var metric v1alpha1.MeterLabelQuery
		var query *PromQuery

		for _, metric = range workload.MetricLabels {
			reqLogger.Info("meterdef preview query ", "metric", metric)
			query = NewPromQuery(&PromQueryArgs{
				Metric: metric.Label,
				Type:   workload.WorkloadType,
				MeterDef: types.NamespacedName{
					Name:      instance.Name,
					Namespace: instance.Namespace,
				},
				Query:         metric.Query,
				Time:          "60m",
				Start:         time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour()-1, time.Now().Minute(), 0, 0, loc),
				End:           time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour(), time.Now().Minute(), 0, 0, loc),
				Step:          time.Hour,
				AggregateFunc: metric.Aggregation,
			})

			reqLogger.Info("meterdef preview query", "query", query.String())

			var warnings v1.Warnings
			err := utils.Retry(func() error {
				var err error
				val, warnings, err = prometheusAPI.ReportQueryWithApi(query)
				if err != nil {
					return errors.Wrap(err, "error with query")
				}

				return nil
			}, *ptr.Int(2))

			if warnings != nil {
				reqLogger.Info("warnings %v", warnings)
			}

			if err != nil {
				reqLogger.Error(err, "prometheus.QueryRange()")
				returnErr = errors.Wrap(err, "error with query")
				return nil, returnErr
			}

			matrix := val.(model.Matrix)
			for _, m := range matrix {
				for _, pair := range m.Values {
					queryPreviewResult = &v1alpha1.Result{
						MetricName: workload.Name,
						Query:      query.String(),
						StartTime:  fmt.Sprintf("%s", query.Start),
						EndTime:    fmt.Sprintf("%s", query.End),
						Value:      int32(pair.Value),
					}
				}
			}

			reqLogger.Info("output", "query preview result", queryPreviewResult)

			if queryPreviewResult != nil {
				queryPreviewResultArray = append(queryPreviewResultArray, *queryPreviewResult)
			}
		}
	}

	return queryPreviewResultArray, nil
}
