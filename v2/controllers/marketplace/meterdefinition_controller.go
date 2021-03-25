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
	"fmt"
	"reflect"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	prom "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const meterDefinitionFinalizer = "meterdefinition.finalizer.marketplace.redhat.com"

const (
	MeteredResourceAnnotationKey = "marketplace.redhat.com/meteredUIDs"
)

// blank assignment to verify that ReconcileMeterDefinition implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterDefinitionReconciler{}

var saClient *prom.ServiceAccountClient

// MeterDefinitionReconciler reconciles a MeterDefinition object
type MeterDefinitionReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client        client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	cfg           *config.OperatorConfig
	cc            ClientCommandRunner
	patcher       patch.Patcher
	kubeInterface kubernetes.Interface
}

func (r *MeterDefinitionReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *MeterDefinitionReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.cc = ccp
	return nil
}

func (r *MeterDefinitionReconciler) InjectPatch(p patch.Patcher) error {
	r.patcher = p
	return nil
}

func (m *MeterDefinitionReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	m.cfg = cfg
	return nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *MeterDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a new controller
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.MeterDefinition{}).
		Watches(&source.Kind{Type: &v1beta1.MeterDefinition{}}, &handler.EnqueueRequestForObject{}).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewMaxOfRateLimiter(
				workqueue.DefaultControllerRateLimiter(),
				workqueue.NewItemExponentialFailureRateLimiter(time.Second, 15*time.Minute),
			),
		}).
		Complete(r)
}

func (r *MeterDefinitionReconciler) InjectKubeInterface(k kubernetes.Interface) error {
	r.kubeInterface = k
	return nil
}

// Reconcile reads that state of the cluster for a MeterDefinition object and makes changes based on the state read
// and what is in the MeterDefinition.Spec
func (r *MeterDefinitionReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterDefinition")

	cc := r.cc

	// Fetch the MeterDefinition instance
	instance := &v1beta1.MeterDefinition{}
	result, _ := cc.Do(context.TODO(),
		GetAction(request.NamespacedName, instance),
	)

	if !result.Is(Continue) {
		if result.Is(NotFound) {
			reqLogger.Info("MeterDef resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}

		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get MeterDef.")
		}

		return result.Return()
	}

	reqLogger.Info("Found instance", "instance", instance.Name)

	var update, requeue bool
	// Set the Status Condition of signature signing
	if instance.IsSigned() {
		// Check the signature again, even though the admission webhook should have
		err := instance.ValidateSignature()
		if err != nil {
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

	if result.Is(Error) {
		reqLogger.Error(result.GetError(), "Failed to update status.")
	}

	queryPreviewResult, err := r.queryPreview(cc, instance, request, reqLogger)
	if err != nil {
		update = update || instance.Status.Conditions.SetCondition(status.Condition{
			Type:    v1beta1.MeterDefQueryPreviewSetupError,
			Reason:  "PreviewError",
			Status:  corev1.ConditionTrue,
			Message: err.Error(),
		})
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
		reqLogger.Info("update required")
		result, _ = cc.Do(context.TODO(), UpdateAction(instance, UpdateStatusOnly(true)))
		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to update status.")
			return result.Return()
		}
	}

	if requeue {
		reqLogger.Info("error happened while trying to generate preview, requeue faster")
		return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	reqLogger.Info("meterdef_preview", "requeue rate", r.cfg.ControllerValues.MeterDefControllerRequeueRate)
	reqLogger.Info("finished reconciling")

	return reconcile.Result{RequeueAfter: r.cfg.ControllerValues.MeterDefControllerRequeueRate}, nil
}

func (r *MeterDefinitionReconciler) finalizeMeterDefinition(req *v1beta1.MeterDefinition) (reconcile.Result, error) {
	var err error

	// TODO: add finalizers

	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), meterDefinitionFinalizer))
	err = r.Client.Update(context.TODO(), req)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// addFinalizer adds finalizers to the MeterDefinition CR
func (r *MeterDefinitionReconciler) addFinalizer(instance *v1beta1.MeterDefinition) error {
	r.Log.Info("Adding Finalizer to %s/%s", instance.Name, instance.Namespace)
	instance.SetFinalizers(append(instance.GetFinalizers(), meterDefinitionFinalizer))

	err := r.Client.Update(context.TODO(), instance)
	if err != nil {
		r.Log.Error(err, "Failed to update RazeeDeployment with the Finalizer %s/%s", instance.Name, instance.Namespace)
		return err
	}
	return nil
}

func (r *MeterDefinitionReconciler) queryPreview(cc ClientCommandRunner, instance *v1beta1.MeterDefinition, request reconcile.Request, reqLogger logr.Logger) ([]common.Result, error) {
	var queryPreviewResult []common.Result

	service, err := r.queryForPrometheusService(context.TODO(), cc, request)
	if err != nil {
		return nil, err
	}

	certConfigMap, err := r.getCertConfigMap(context.TODO(), cc, request)
	if err != nil {
		return nil, err
	}

	saClient := prom.NewServiceAccountClient(r.cfg.ControllerValues.DeploymentNamespace, r.kubeInterface)

	authToken, err := saClient.NewServiceAccountToken(utils.OPERATOR_SERVICE_ACCOUNT, utils.PrometheusAudience, 3600, reqLogger)
	if err != nil {
		return nil, err
	}

	if certConfigMap != nil && authToken != "" && service != nil {
		cert, err := parseCertificateFromConfigMap(*certConfigMap)
		if err != nil {
			return nil, err
		}

		prometheusAPI, err := prom.NewPromAPI(service, &cert, authToken)
		if err != nil {
			return nil, err
		}

		reqLogger.Info("generatring meterdef preview")
		return generateQueryPreview(instance, prometheusAPI, reqLogger)
	}

	return queryPreviewResult, nil
}

func (r *MeterDefinitionReconciler) queryForPrometheusService(
	ctx context.Context,
	cc ClientCommandRunner,
	req reconcile.Request,
) (*corev1.Service, error) {
	service := &corev1.Service{}

	name := types.NamespacedName{
		Name:      utils.PROMETHEUS_METERBASE_NAME,
		Namespace: r.cfg.DeployedNamespace,
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		return nil, errors.Wrap(result, "failed to get prometheus service")
	}

	log.Info("retrieved prometheus service")
	return service, nil
}

func (r *MeterDefinitionReconciler) getCertConfigMap(ctx context.Context, cc ClientCommandRunner, req reconcile.Request) (*corev1.ConfigMap, error) {
	certConfigMap := &corev1.ConfigMap{}

	name := types.NamespacedName{
		Name:      utils.OPERATOR_CERTS_CA_BUNDLE_NAME,
		Namespace: r.cfg.ControllerValues.DeploymentNamespace,
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

func returnQueryRange(duration time.Duration) (startTime time.Time, endTime time.Time) {
	endTime = time.Now().UTC().Truncate(time.Hour)
	startTime = endTime.Add(-duration)
	return
}

func generateQueryPreview(instance *v1beta1.MeterDefinition, prometheusAPI *prom.PrometheusAPI, reqLogger logr.Logger) (queryPreviewResultArray []common.Result, returnErr error) {
	var queryPreviewResult *common.Result
	labels := instance.ToPrometheusLabels()

	for _, meterWorkload := range labels {
		var val model.Value
		startTime, endTime := returnQueryRange(meterWorkload.MetricPeriod.Duration)
		query := prom.PromQueryFromLabels(meterWorkload, startTime, endTime)

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
			reqLogger.Info("warnings %v", warnings)
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

func labelsForServiceMonitor(name, namespace string) map[string]string {
	return map[string]string{
		"marketplace.redhat.com/metered":                  "true",
		"marketplace.redhat.com/deployed":                 "true",
		"marketplace.redhat.com/metered.kind":             "ServiceMonitor",
		"marketplace.redhat.com/serviceMonitor.Name":      name,
		"marketplace.redhat.com/serviceMonitor.Namespace": namespace,
	}
}

func labelsForKubeStateMonitor(name, namespace string) map[string]string {
	return map[string]string{
		"marketplace.redhat.com/metered":                   "true",
		"marketplace.redhat.com/deployed":                  "true",
		"marketplace.redhat.com/metered.kind":              "ServiceMonitor",
		"marketplace.redhat.com/meterDefinition.namespace": namespace,
		"marketplace.redhat.com/meterDefinition.name":      name,
	}
}

func makeRelabelConfig(source []string, action, target string) *monitoringv1.RelabelConfig {
	return &monitoringv1.RelabelConfig{
		SourceLabels: source,
		TargetLabel:  target,
		Action:       action,
	}
}

func makeRelabelReplaceConfig(source []string, target, regex, replacement string) *monitoringv1.RelabelConfig {
	return &monitoringv1.RelabelConfig{
		SourceLabels: source,
		TargetLabel:  target,
		Action:       "replace",
		Regex:        regex,
		Replacement:  replacement,
	}
}

func makeRelabelKeepConfig(source []string, regex string) *monitoringv1.RelabelConfig {
	return &monitoringv1.RelabelConfig{
		SourceLabels: source,
		Action:       "keep",
		Regex:        regex,
	}

}
func labelsToRegex(labels []string) string {
	return fmt.Sprintf("(%s)", strings.Join(labels, "|"))
}
