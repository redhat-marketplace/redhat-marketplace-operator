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

package meterdefinition

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"emperror.dev/errors"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	"github.com/operator-framework/operator-sdk/pkg/status"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	v1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/meter_definition"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/prometheus"

	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const meterDefinitionFinalizer = "meterdefinition.finalizer.marketplace.redhat.com"

const (
	MeteredResourceAnnotationKey = "marketplace.redhat.com/meteredUIDs"
)

var (
	log = logf.Log.WithName("controller_meterdefinition")
	// uid to name and namespace
	store *meter_definition.MeterDefinitionStore

	saClient *prometheus.ServiceAccountClient
)

// Add creates a new MeterDefinition Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
	kubernetesInterface kubernetes.Interface,
	cfg config.OperatorConfig,
) error {
	return add(mgr, newReconciler(mgr, ccprovider, kubernetesInterface, cfg))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, ccprovider ClientCommandRunnerProvider, kubernetesInterface kubernetes.Interface, cfg config.OperatorConfig) reconcile.Reconciler {
	opts := &MeterDefOpts{}
	
	return &ReconcileMeterDefinition{
		client:               mgr.GetClient(),
		kubernetesInterface : kubernetesInterface,
		scheme:               mgr.GetScheme(),
		ccprovider:           ccprovider,
		opts:                 opts,
		patcher:              patch.RHMDefaultPatcher,
		cfg:                  cfg,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("meterdefinition-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MeterDefinition
	err = c.Watch(&source.Kind{Type: &v1alpha1.MeterDefinition{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return err
}

// blank assignment to verify that ReconcileMeterDefinition implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMeterDefinition{}

// ReconcileMeterDefinition reconciles a MeterDefinition object
type ReconcileMeterDefinition struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client               client.Client
	kubernetesInterface  kubernetes.Interface
	scheme               *runtime.Scheme
	ccprovider           ClientCommandRunnerProvider
	opts                 *MeterDefOpts
	patcher              patch.Patcher
	cfg                  config.OperatorConfig
}

type MeterDefOpts struct{}

// Reconcile reads that state of the cluster for a MeterDefinition object and makes changes based on the state read
// and what is in the MeterDefinition.Spec
func (r *ReconcileMeterDefinition) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	reqLogger.Info("Reconciling MeterDefinition")

	cc := r.ccprovider.NewCommandRunner(r.client, r.scheme, reqLogger)

	// Fetch the MeterDefinition instance
	instance := &v1alpha1.MeterDefinition{}
	result, _ := cc.Do(context.TODO(), GetAction(request.NamespacedName, instance))

	if !result.Is(Continue) {
		if !result.Is(NotFound) {
			reqLogger.Info("MeterDef resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}

		if !result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get MeterDef.")
		}

		return result.Return()
	}

	reqLogger.Info("Found instance", "instance", instance.Name)

	var update bool

	if instance.Spec.ServiceMeterLabels != nil {
		instance.Spec.ServiceMeterLabels = nil
	}
	if instance.Spec.PodMeterLabels != nil {
		instance.Spec.PodMeterLabels = nil
	}

	switch {
	case instance.Status.Conditions.IsUnknownFor(v1alpha1.MeterDefConditionTypeHasResult):
		fallthrough
	case len(instance.Status.WorkloadResources) == 0:
		update = instance.Status.Conditions.SetCondition(v1alpha1.MeterDefConditionNoResults)
	case len(instance.Status.WorkloadResources) > 0:
		update = instance.Status.Conditions.SetCondition(v1alpha1.MeterDefConditionHasResults)
	}

	queryPreviewResult,err := r.queryPreview(cc,instance,request,reqLogger)
	if err != nil {
		update = instance.Status.Conditions.SetCondition(status.Condition{
			Type: v1alpha1.MeterDefQueryPreviewSetupError,
			Message: err.Error(),
		})
	}else if err == nil {
		update = instance.Status.Conditions.RemoveCondition(v1alpha1.MeterDefQueryPreviewSetupError)
	}

	if !reflect.DeepEqual(queryPreviewResult, instance.Status.Results) {
		instance.Status.Results = queryPreviewResult
		reqLogger.Info("output", "Status.Results", instance.Status.Results)
		update = true
	}

	result, _ = cc.Do(
		context.TODO(),
		Call(func() (ClientAction, error) {
			if !update {
				return nil, nil
			}

			return UpdateAction(instance, UpdateStatusOnly(true)), nil
		}),
	)
	if result.Is(Error) {
		reqLogger.Error(result.GetError(), "Failed to update status.")
	}

	reqLogger.Info("meterdef_preview","requeue rate",r.cfg.ControllerValues.MeterDefControllerRequeueRate)

	reqLogger.Info("finished reconciling")
	return reconcile.Result{RequeueAfter: r.cfg.ControllerValues.MeterDefControllerRequeueRate}, nil
}

func (r *ReconcileMeterDefinition) queryPreview(cc ClientCommandRunner, instance *v1alpha1.MeterDefinition, request reconcile.Request, reqLogger logr.Logger) ([]v1alpha1.Result,error) {
	var queryPreviewResult []v1alpha1.Result

	service, err := r.queryForPrometheusService(context.TODO(), cc, request)
	if err != nil {
		return nil,err
	}

	certConfigMap, err := r.getCertConfigMap(context.TODO(), cc, request)
	if err != nil {
		return nil,err
	}

	saClient := NewServiceAccountClient(r.cfg.ControllerValues.DeploymentNamespace, r.kubernetesInterface)

	authToken, err := saClient.NewServiceAccountToken(utils.OPERATOR_SERVICE_ACCOUNT, utils.PrometheusAudience, 3600, reqLogger)
	if err != nil {
		return nil,err
	}

	if certConfigMap != nil && authToken != "" && service != nil {
		cert, err := parseCertificateFromConfigMap(*certConfigMap)
		if err != nil {
			return nil,err
		}

		prometheusAPI,err := NewPromAPI(service,&cert,authToken)
		if err != nil {
			return nil,err
		}

		reqLogger.Info("generatring meterdef preview")
		queryPreviewResult, err = generateQueryPreview(instance,prometheusAPI ,reqLogger)
		if err != nil {
			return nil,err
		}
	}

	return queryPreviewResult,nil
}

func (r *ReconcileMeterDefinition) queryForPrometheusService(
	ctx context.Context,
	cc ClientCommandRunner,
	req reconcile.Request,
) (*corev1.Service, error) {
	service := &corev1.Service{}

	name := types.NamespacedName{
		Name:      utils.PROMETHEUS_METERBASE_NAME,
		Namespace: 	r.cfg.ControllerValues.DeploymentNamespace,
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		return nil, errors.Wrap(result, "failed to get prometheus service")
	}

	log.Info("retrieved prometheus service")
	return service, nil
}

func (r *ReconcileMeterDefinition) getCertConfigMap(ctx context.Context, cc ClientCommandRunner, req reconcile.Request) (*corev1.ConfigMap, error) {
	certConfigMap := &corev1.ConfigMap{}

	name := types.NamespacedName{
		Name:      utils.OPERATOR_CERTS_CA_BUNDLE_NAME,
		Namespace: 	r.cfg.ControllerValues.DeploymentNamespace,
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


func returnQueryRange()(startTime time.Time, endTime time.Time){
	baseTime := time.Now().Truncate(time.Minute)
	end := baseTime
	start := end.Add(-time.Hour)
	return start,end
}

func generateQueryPreview(instance *v1alpha1.MeterDefinition, prometheusAPI *PrometheusAPI, reqLogger logr.Logger) (queryPreviewResultArray []v1alpha1.Result, returnErr error) {

	startTime, endTime := returnQueryRange()
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
				Start:         startTime,
				End:           endTime,
				Step:          time.Hour,
				AggregateFunc: metric.Aggregation,
			})

			reqLogger.Info("meterdef preview query", "query", query.String())

			var warnings v1.Warnings
			err := utils.Retry(func() error {
				var err error
				val, warnings, err = prometheusAPI.ReportQuery(query)
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

func (r *ReconcileMeterDefinition) finalizeMeterDefinition(req *v1alpha1.MeterDefinition) (reconcile.Result, error) {
	var err error

	// TODO: add finalizers

	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), meterDefinitionFinalizer))
	err = r.client.Update(context.TODO(), req)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// addFinalizer adds finalizers to the MeterDefinition CR
func (r *ReconcileMeterDefinition) addFinalizer(instance *v1alpha1.MeterDefinition) error {
	log.Info("Adding Finalizer to %s/%s", instance.Name, instance.Namespace)
	instance.SetFinalizers(append(instance.GetFinalizers(), meterDefinitionFinalizer))

	err := r.client.Update(context.TODO(), instance)
	if err != nil {
		log.Error(err, "Failed to update RazeeDeployment with the Finalizer %s/%s", instance.Name, instance.Namespace)
		return err
	}
	return nil
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
