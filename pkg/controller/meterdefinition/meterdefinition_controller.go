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
	"github.com/gotidy/ptr"
	corev1 "k8s.io/api/core/v1"

	// promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	v1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/meter_definition"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

var log = logf.Log.WithName("controller_meterdefinition")

// uid to name and namespace
var store *meter_definition.MeterDefinitionStore

// Add creates a new MeterDefinition Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
	queryForPrometheus QueryForPrometheusServiceFunc,
	provideAPIClient ProvideAPIClientFunc,
) error {
	return add(mgr, newReconciler(mgr, ccprovider, queryForPrometheus, provideAPIClient))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, ccprovider ClientCommandRunnerProvider, queryForPrometheus QueryForPrometheusServiceFunc, provideAPIClient ProvideAPIClientFunc) reconcile.Reconciler {
	opts := &MeterDefOpts{}

	return &ReconcileMeterDefinition{
		client:             mgr.GetClient(),
		scheme:             mgr.GetScheme(),
		ccprovider:         ccprovider,
		queryForPrometheus: queryForPrometheus,
		provideAPIClient:   provideAPIClient,
		opts:               opts,
		patcher:            patch.RHMDefaultPatcher,
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
	client             client.Client
	scheme             *runtime.Scheme
	ccprovider         ClientCommandRunnerProvider
	queryForPrometheus QueryForPrometheusServiceFunc
	provideAPIClient   ProvideAPIClientFunc
	opts               *MeterDefOpts
	patcher            patch.Patcher
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

	var queue bool

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
		queue = instance.Status.Conditions.SetCondition(v1alpha1.MeterDefConditionNoResults)
	case len(instance.Status.WorkloadResources) > 0:
		queue = instance.Status.Conditions.SetCondition(v1alpha1.MeterDefConditionHasResults)
	}

	if r.queryForPrometheus == nil {
		reqLogger.Info("queryForProm", "setup", "is not present")
	}

	service, err := r.queryForPrometheus(context.TODO(), cc)
	if err != nil {
		reqLogger.Error(err, "error encountered")
	}

	if service == nil {
		reqLogger.Info("SERVICE IS NIL")
	}

	reqLogger.Info("Info", "SERVICE NAME", service.Name)

	//TODO: query the cm for cert
	certConfigMap := &corev1.ConfigMap{}

	name := types.NamespacedName{
		Name:      "operator-certs-ca-bundle",
		Namespace: "openshift-redhat-marketplace",
	}

	if result, _ := cc.Do(context.TODO(), GetAction(name, certConfigMap)); !result.Is(Continue) {
		reqLogger.Error(result.GetError(), "Failed to update status.")
	}

	// get cert from ConfigMap
	dataMap := &certConfigMap.Data
	var cert []byte
	for _,value := range *dataMap{
		b := []byte(value)
		cert = b
	}

	reqLogger.Info("Info","cert",cert)

	client, err := r.provideAPIClient(service,&cert)
	loc, _ := time.LoadLocation("UTC")

	if client == nil {
		reqLogger.Info("CLIENT IS NIL")
	}

	reqLogger.Info("Info","client",client)

	// var promAPI v1.API
	promAPI := v1.NewAPI(client)
	reqLogger.Info("Info", "return prom api", promAPI)

	if promAPI == nil {
		reqLogger.Info("PROM API IS NIL")
	}

	var queryPreviewResult *v1alpha1.Result
	var queryPreviewResultArray []v1alpha1.Result

	for _, workload := range instance.Spec.Workloads {
		var val model.Value
		var metric v1alpha1.MeterLabelQuery
		var query *prometheus.PromQuery

		for _, metric = range workload.MetricLabels {
			reqLogger.Info("query", "metric", metric)
			query = &prometheus.PromQuery{
				Metric: metric.Label,
				Type:   workload.WorkloadType,
				MeterDef: types.NamespacedName{
					Name:      instance.Name,
					Namespace: instance.Namespace,
				},
				Query:         metric.Query,
				Time:          "60m",
				Start:         time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour(), time.Now().Minute()-1, 0, 0, loc),
				End:           time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour(), time.Now().Minute(), 0, 0, loc),
				Step:          time.Hour,
				AggregateFunc: metric.Aggregation,
			}

			reqLogger.Info("output", "query", query.String())

			var warnings v1.Warnings
			err := utils.Retry(func() error {
				var err error
				val, warnings, err = prometheus.QueryRange(query, promAPI)

				if err != nil {
					return errors.Wrap(err, "error with query")
				}

				return nil
			}, *ptr.Int(2))

			if warnings != nil {
				reqLogger.Info("warnings %v", warnings)
			}

			if err != nil {
				reqLogger.Error(err, "error encountered")
				return reconcile.Result{}, err
			}

			matrix := val.(model.Matrix)
			for _,m := range matrix{
				for _,pair := range m.Values {
					queryPreviewResult = &v1alpha1.Result{
						WorkloadName: workload.Name,
						QueryName: metric.Label,
						StartTime: fmt.Sprintf("%s",query.Start),
						EndTime: fmt.Sprintf("%s",query.End),
						Value: int32(pair.Value),
					}
				}
			}

			if queryPreviewResult != nil{
				reqLogger.Info("output", "query preview result", queryPreviewResult)
				queryPreviewResultArray = append(queryPreviewResultArray, *queryPreviewResult)
			}
		}
	}

	if !reflect.DeepEqual(queryPreviewResultArray,instance.Status.Results) {
		queue = true
		instance.Status.Results = queryPreviewResultArray
		reqLogger.Info("output","Status.Results",instance.Status.Results)
	}

	result, _ = cc.Do(
		context.TODO(),
		Call(func() (ClientAction, error) {
			if !queue {
				return nil, nil
			}

			return UpdateAction(instance, UpdateStatusOnly(true)), nil
		}),
	)
	if result.Is(Error) {
		reqLogger.Error(result.GetError(), "Failed to update status.")
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{RequeueAfter: time.Second * 5}, nil
}

// func (queryPreview v1alpha1.Result) IsEmpty() bool {
// 	return reflect.DeepEqual(queryPreview,v1alpha1.Result{})
// }

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
