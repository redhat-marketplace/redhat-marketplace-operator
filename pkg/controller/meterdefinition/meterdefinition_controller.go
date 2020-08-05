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
	"strings"
	"time"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	v1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/meter_definition"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/runtime"
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
) error {
	return add(mgr, newReconciler(mgr, ccprovider))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, ccprovider ClientCommandRunnerProvider) reconcile.Reconciler {
	opts := &MeterDefOpts{}

	return &ReconcileMeterDefinition{
		client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		ccprovider: ccprovider,
		opts:       opts,
		patcher:    patch.RHMDefaultPatcher,
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

	err = c.Watch(&source.Kind{Type: &monitoringv1.ServiceMonitor{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.MeterDefinition{},
	})

	return err
}

// blank assignment to verify that ReconcileMeterDefinition implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMeterDefinition{}

// ReconcileMeterDefinition reconciles a MeterDefinition object
type ReconcileMeterDefinition struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	ccprovider ClientCommandRunnerProvider
	opts       *MeterDefOpts
	patcher    patch.Patcher
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

	// Adding a finalizer to this CR
	if !utils.Contains(instance.GetFinalizers(), meterDefinitionFinalizer) {
		if err := r.addFinalizer(instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Check if the MeterDefinition instance is being marked for deletion
	isMarkedForDeletion := instance.GetDeletionTimestamp() != nil
	if isMarkedForDeletion {
		if utils.Contains(instance.GetFinalizers(), meterDefinitionFinalizer) {
			//Run finalization logic for the MeterDefinitionFinalizer.
			//If it fails, don't remove the finalizer so we can retry during the next reconcile
			return r.finalizeMeterDefinition(instance)
		}
		return reconcile.Result{}, nil
	}

	// var namespaces []string

	// switch instance.Spec.WorkloadVertex {
	// case v1alpha1.WorkloadVertexOperatorGroup:
	// 	reqLogger.Info("operatorGroup vertex")
	// 	csv := &olmv1.ClusterServiceVersion{}

	// 	if instance.Spec.InstalledBy == nil {
	// 		reqLogger.Info("installed by not found", "meterdef", instance.Name+"/"+instance.Namespace)

	// 		return result.Return()
	// 	}

	// 	result, _ := cc.Do(context.TODO(),
	// 		GetAction(instance.Spec.InstalledBy.ToTypes(), csv),
	// 	)

	// 	if !result.Is(Continue) {
	// 		// TODO: set condition and requeue later, may be too early
	// 		reqLogger.Info("csv not found", "csv", instance.Spec.InstalledBy)

	// 		return result.Return()
	// 	}

	// 	olmNamespacesStr, ok := csv.GetAnnotations()["olm.targetNamespaces"]

	// 	if !ok {
	// 		// set condition and requeue for later
	// 		reqLogger.Info("olmNamespaces not found")
	// 		return result.Return()
	// 	}

	// 	if olmNamespacesStr == "" {
	// 		reqLogger.Info("operatorGroup is for all namespaces")
	// 		namespaces = []string{corev1.NamespaceAll}
	// 		break
	// 	}

	// 	namespaces = strings.Split(olmNamespacesStr, ",")
	// case v1alpha1.WorkloadVertexNamespace:
	// 	reqLogger.Info("namespace vertex with filter")

	// 	if instance.Spec.VertexLabelSelectors == nil || len(instance.Spec.VertexLabelSelectors) == 0 {
	// 		reqLogger.Info("namespace vertex is for all namespaces")
	// 		break
	// 	}

	// 	namespaceList := &corev1.NamespaceList{}

	// 	result, _ := cc.Do(context.TODO(),
	// 		ListAction(namespaceList, instance.Spec.VertexLabelSelector),
	// 	)

	// 	if !result.Is(Continue) {
	// 		// TODO: set condition and requeue later, may be too early
	// 		reqLogger.Info("csv not found", "csv", instance.Spec.InstalledBy)

	// 		return result.Return()
	// 	}

	// 	for _, ns := range namespaceList {
	// 		namespaces = append(namespaces, ns.GetName())
	// 	}
	// }

	// if len(namespaces) == 0 {
	// 	reqLogger.Info("no namespaces found to filter on, will quit")

	// 	//TODO: set condition
	// 	//
	// 	return reconcile.Result{RequeueAfter: 30 * time.Minute}, nil
	// }

	// reqLogger.Info("found namespaces", "namespaces", namespaces)

	// // find pods, services, service monitors, and pvcs
	// // annotate meterdef with these finds, also include in status
	// // use annotations to drive the metric service

	// pods := []*corev1.Pod{}
	// serviceMonitors := []*monitoringv1.ServiceMonitor{}
	// services := []*corev1.Service{}
	// pvcs := []*corev1.PersistentVolumeClaim{}

	// // two strategies. Bottom up, or top down.

	// // Bottom Up
	// // Start with pods, filter, go to owner. If owner not provided, stop.

	// namespaceOptions := []client.ListOption{}

	// for _, ns := range namespaces {
	// 	for _, workload := range instance.Spec.Workloads {
	// 		if workload.Owner != nil {
	// 			// easier lookup
	// 		}

	// 		// harder lookup
	// 		listOptions := []client.ListOption{client.InNamespace(ns)}

	// 		if workload.LabelSelector != nil {
	// 			listOptions = append(listOptions, client.MatchingLabelsSelector{Selector: workload.Labels})
	// 		}

	// 		if workload.AnnotationSelector != nil {
	// 			fieldSelector := fields.Set{}
	// 			for key, val := range workload.AnnotationSelector.MatchAnnotations {
	// 				fieldSelector[rhmclient.IndexAnnotations] = fmt.Sprintf("%s=%s", key, value)
	// 			}
	// 			listOptions = append(listOptions, client.MatchingFieldsSelector{Selector: fieldSelector})
	// 		}

	// 		var lookupList []runtime.Object
	// 		switch workload.WorkloadType {
	// 		case WorkloadTypePod:
	// 			// find pods
	// 			lookupList = pods
	// 		case WorkloadTypeServiceMonitor:
	// 			// find service monitors and services
	// 			lookupList = serviceMonitors
	// 		case WorkloadTypePVC:
	// 			// find pvcs attached to pods
	// 			lookupList = pvcs
	// 		}

	// 		result, _ := cc.Do(
	// 			context.TODO(),
	// 			ListAppendAction(pods,
	// 				client.MatchingFieldsSelector{Selector: workload.AnnotationSelector},
	// 				listOptions...,
	// 			),
	// 		)

	// 		if !result.Is(Continue) {
	// 			if result.Is(Error) {
	// 				reqLogger.Error(result, "failed to build list")
	// 			}
	// 			return result.Return()
	// 		}
	// 	}
	// }

	// serviceMonitorReferences := []common.NamespacedNameReference{}

	// // Find services from service monitors
	// for _, sm := range serviceMonitors {
	// 	result, _ := cc.Do(
	// 		context.TODO(),
	// 		ListAppendAction(
	// 			services,
	// 			client.MatchingLabelsSelector{Selector: sm.Spec.Selector},
	// 		),
	// 	)
	// 	if !result.Is(Continue) {
	// 		if result.Is(Error) {
	// 			reqLogger.Error(result, "failed to build list")
	// 		}
	// 		return result.Return()
	// 	}

	// 	serviceMonitorReferences = append(serviceMonitorReferences, common.NamespacedNameFromMeta(sm))
	// }

	// // Collector meteredUIDs for annotations and status; store in our cache
	// meteredUIDs := []types.UID{}

	// for _, p := range pods {
	// 	meteredUIDs = append(meteredUIDs, p.UID)
	// 	ownerMap.Store(instance.UID, p.UID, &request)
	// }

	// for _, s := range services {
	// 	meteredUIDs = append(meteredUIDs, s.UID)
	// 	ownerMap.Store(instance.UID, s.UID, &request)
	// }

	// for _, pv := range pvcs {
	// 	meteredUIDs = append(meteredUIDs, pv.UID)
	// 	ownerMap.Store(instance.UID, pv.UID, &request)
	// }

	// // set annotations and status
	// sort.Sort(meteredUIDs)
	// ogInstance = instance.DeepCopy()
	// instance.Annotations[MeteredResourceAnnotationKey] = strings.Join(meteredUIDs, ",")
	// instance.Status.ServiceMonitors = serviceMonitorReferences

	// patch, err := r.patcher.Calculate(ogInstance, instance)

	// if err != nil {
	// 	return reconcile.Result{}, err
	// }

	// patchBytes, err := jsonpatch.CreateMergePatch(patch.Original, patch.Modified)

	// if err != nil {
	// 	return reconcile.Result{}, err
	// }

	// result, _ = cc.Do(context.TODO(),
	// 	UpdateWithPatchAction(instance, types.MergePatchType, patchBytes),
	// )

	// if !result.Is(Continue) {
	// 	if result.Is(Error) {
	// 		reqLogger.Error(result, "failed to build list")
	// 	}
	// 	return result.Return()
	// }

	//
	// loop over workloads using vertex to find dependents.
	//
	//

	// gvkStr := strings.ToLower(fmt.Sprintf("%s.%s.%s", instance.Spec.Kind, instance.Spec.Version, instance.Spec.Group))

	// podRefs := []*common.PodReference{}
	// podList := &corev1.PodList{}
	// replicaSetList := &appsv1.ReplicaSetList{}
	// deploymentList := &appsv1.DeploymentList{}
	// statefulsetList := &appsv1.StatefulSetList{}
	// daemonsetList := &appsv1.DaemonSetList{}
	// podLookupStrings := []string{gvkStr}
	// serviceMonitors := &monitoringv1.ServiceMonitorList{}

	// result, _ = cc.Do(context.TODO(),
	// 	HandleResult(
	// 		ListAction(deploymentList, client.MatchingFields{rhmclient.OwnerRefContains: gvkStr}),
	// 		OnContinue(Call(func() (ClientAction, error) {
	// 			actions := []ClientAction{}

	// 			for _, depl := range deploymentList.Items {
	// 				actions = append(actions, ListAppendAction(replicaSetList, client.MatchingField(rhmclient.OwnerRefContains, string(depl.UID))))
	// 			}
	// 			return Do(actions...), nil
	// 		})),
	// 	),
	// 	HandleResult(
	// 		Do(
	// 			ListAppendAction(replicaSetList, client.MatchingField(rhmclient.OwnerRefContains, gvkStr)),
	// 			ListAction(statefulsetList, client.MatchingField(rhmclient.OwnerRefContains, gvkStr)),
	// 			ListAction(daemonsetList, client.MatchingField(rhmclient.OwnerRefContains, gvkStr)),
	// 			ListAction(serviceMonitors, client.MatchingField(rhmclient.OwnerRefContains, gvkStr)),
	// 		),
	// 		OnContinue(Call(func() (ClientAction, error) {
	// 			for _, rs := range replicaSetList.Items {
	// 				podLookupStrings = append(podLookupStrings, string(rs.UID))
	// 			}

	// 			for _, item := range statefulsetList.Items {
	// 				podLookupStrings = append(podLookupStrings, string(item.UID))
	// 			}

	// 			for _, item := range daemonsetList.Items {
	// 				podLookupStrings = append(podLookupStrings, string(item.UID))
	// 			}

	// 			actions := []ClientAction{}

	// 			for _, lookup := range podLookupStrings {
	// 				actions = append(actions, ListAppendAction(podList, client.MatchingFields{rhmclient.OwnerRefContains: lookup}))
	// 			}

	// 			return Do(actions...), nil
	// 		}))),
	// )

	// if !result.Is(Continue) {
	// 	if result.Is(Error) {
	// 		reqLogger.Error(result, "failed to get lists")
	// 	}
	// 	return result.Return()
	// }

	// for _, p := range podList.Items {
	// 	pr := &common.PodReference{}
	// 	pr.FromPod(&p)
	// 	podRefs = append(podRefs, pr)
	// }

	// reqLogger.Info("some data", "refs", podRefs, "serviceMonitors", serviceMonitors)

	// if !reflect.DeepEqual(instance.Spec.Pods, podRefs) {
	// 	instance.Spec.Pods = podRefs
	// 	result, _ = cc.Do(context.TODO(), UpdateAction(instance))

	// 	if !result.Is(Continue) {
	// 		if result.Is(Error) {
	// 			reqLogger.Error(result, "failed to get lists")
	// 		}
	// 		return result.Return()
	// 	}
	// }

	reqLogger.Info("finished reconciling")
	return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
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

// func configureServiceMonitorFromMeterLabels(def *v1alpha1.MeterDefinition, monitor *monitoringv1.ServiceMonitor) {
// 	endpoints := []monitoringv1.Endpoint{}
// 	for _, endpoint := range monitor.Spec.Endpoints {
// 		newEndpoint := endpoint.DeepCopy()
// 		relabelConfigs := []*monitoringv1.RelabelConfig{
// 			makeRelabelReplaceConfig([]string{"__name__"}, "meter_kind", "(.*)", def.Spec.Kind),
// 			makeRelabelReplaceConfig([]string{"__name__"}, "meter_domain", "(.*)", def.Spec.Group),
// 		}
// 		metricRelabelConfigs := []*monitoringv1.RelabelConfig{
// 			makeRelabelKeepConfig([]string{"__name__"}, labelsToRegex(def.Spec.ServiceMeters)),
// 		}
// 		newEndpoint.RelabelConfigs = append(newEndpoint.RelabelConfigs, relabelConfigs...)
// 		newEndpoint.MetricRelabelConfigs = metricRelabelConfigs
// 		endpoints = append(endpoints, *newEndpoint)
// 	}
// 	monitor.Spec.Endpoints = endpoints
// }

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

