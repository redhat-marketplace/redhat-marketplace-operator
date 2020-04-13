package meterdefinition

import (
	"context"
	"fmt"
	"strings"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/utils"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const meterDefinitionFinalizer = "meterdefinition.finalizer.marketplace.redhat.com"

var log = logf.Log.WithName("controller_meterdefinition")

// Add creates a new MeterDefinition Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	opts := &MeterDefOpts{}

	return &ReconcileMeterDefinition{client: mgr.GetClient(), scheme: mgr.GetScheme(), opts: opts}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("meterdefinition-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MeterDefinition
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MeterDefinition{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &monitoringv1.ServiceMonitor{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterDefinition{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMeterDefinition implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMeterDefinition{}

// ReconcileMeterDefinition reconciles a MeterDefinition object
type ReconcileMeterDefinition struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	opts   *MeterDefOpts
}

type MeterDefOpts struct{}

// Reconcile reads that state of the cluster for a MeterDefinition object and makes changes based on the state read
// and what is in the MeterDefinition.Spec
func (r *ReconcileMeterDefinition) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterDefinition")

	// Fetch the MeterDefinition instance
	instance := &marketplacev1alpha1.MeterDefinition{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("could not find the meter def")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "error looking for meterdef")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Found instance", "instance", instance.Name)

	// Adding a finalizer to this CR
	if !utils.Contains(instance.GetFinalizers(), meterDefinitionFinalizer) {
		if err := r.addFinalizer(instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Check if the RazeeDeployment instance is being marked for deletion
	isMarkedForDeletion := instance.GetDeletionTimestamp() != nil
	if isMarkedForDeletion {
		if utils.Contains(instance.GetFinalizers(), meterDefinitionFinalizer) {
			//Run finalization logic for the razeeDeploymentFinalizer.
			//If it fails, don't remove the finalizer so we can retry during the next reconcile
			return r.finalizeMeterDefinition(instance)
		}
		return reconcile.Result{}, nil
	}

	// ---
	// Collect current state
	// ---
	serviceMonitorList := &monitoringv1.ServiceMonitorList{}

	serviceMonitorMatchLabels := &metav1.LabelSelector{}

	if instance.Spec.ServiceMonitorSelector == nil {
		reqLogger.Info("instance does not have any filters, no-op")
		return reconcile.Result{}, nil
	}

	if instance.Spec.ServiceMonitorSelector != nil {
		serviceMonitorMatchLabels = instance.Spec.ServiceMonitorSelector
	}

	// TODO: Add check for empty match
	// TODO: Add namespace filter
	reqLogger.Info("looking for service monitors with labels", "labels", serviceMonitorMatchLabels.MatchLabels)

	listOpts := []client.ListOption{
		client.MatchingLabels(serviceMonitorMatchLabels.MatchLabels),
	}
	err = r.client.List(context.TODO(), serviceMonitorList, listOpts...)

	if err != nil {
		reqLogger.Error(err, "Failed to list service monitors.",
			"MeterBase.Namespace", instance.Namespace,
			"MeterBase.Name", instance.Name)
		return reconcile.Result{}, err
	}

	reqLogger.Info("retreived service monitors in scope of def", "size", len(serviceMonitorList.Items))

	// TODO: Add labels
	// TODO: Add namespace filter
	podMonitorMatchLabels := &metav1.LabelSelector{}

	if instance.Spec.PodSelector != nil {
		podMonitorMatchLabels = instance.Spec.PodSelector
	}

	podList := &corev1.PodList{}
	listOpts = []client.ListOption{
		client.MatchingLabels(podMonitorMatchLabels.MatchLabels),
	}
	err = r.client.List(context.TODO(), podList, listOpts...)

	if err != nil {
		reqLogger.Error(err, "Failed to list posd.",
			"MeterBase.Namespace", instance.Namespace,
			"MeterBase.Name", instance.Name)
		return reconcile.Result{}, err
	}

	// we'll use labels to identify what we create
	//
	meteredServiceMonitors := &monitoringv1.ServiceMonitorList{}
	listOpts = []client.ListOption{
		client.MatchingLabels(map[string]string{
			"marketplace.redhat.com/metered":      "true",
			"marketplace.redhat.com/deployed":     "true",
			"marketplace.redhat.com/metered.kind": "ServiceMonitor",
		}),
		client.InNamespace(instance.Namespace),
	}
	err = r.client.List(context.TODO(), meteredServiceMonitors, listOpts...)

	if err != nil {
		reqLogger.Error(err, "Failed to list service monitors.",
			"MeterBase.Namespace", instance.Namespace,
			"MeterBase.Name", instance.Name)
		return reconcile.Result{}, err
	}

	meteredPodList := &corev1.PodList{}
	listOpts = []client.ListOption{
		client.MatchingLabels(map[string]string{
			"marketplace.redhat.com/metered":      "true",
			"marketplace.redhat.com/metered.kind": "Pod",
		}),
	}
	err = r.client.List(context.TODO(), meteredPodList, listOpts...)

	if err != nil {
		reqLogger.Error(err, "Failed to list posd.",
			"MeterBase.Namespace", instance.Namespace,
			"MeterBase.Name", instance.Name)
		return reconcile.Result{}, err
	}

	// find specific service monitor for kube-state
	openshiftKubeStateMonitor := &monitoringv1.ServiceMonitor{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: "openshift-monitoring",
		Name:      "kube-state-metrics",
	}, openshiftKubeStateMonitor)
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		reqLogger.Info("can't find openshift kube state")
	}

	kubeStateServiceMonitors := &monitoringv1.ServiceMonitorList{}
	listOpts = []client.ListOption{
		client.MatchingLabels(map[string]string{
			"marketplace.redhat.com/metered":                   "true",
			"marketplace.redhat.com/metered.kind":              "ServiceMonitor",
			"marketplace.redhat.com/meterDefinition.namespace": instance.Namespace,
			"marketplace.redhat.com/meterDefinition.name":      instance.Name,
		}),
		client.InNamespace(instance.Namespace),
	}
	err = r.client.List(context.TODO(), kubeStateServiceMonitors, listOpts...)

	if err != nil {
		reqLogger.Error(err, "Failed to list service monitors.",
			"MeterBase.Namespace", instance.Namespace,
			"MeterBase.Name", instance.Name)
		return reconcile.Result{}, err
	}

	//---
	// Reconcile service monitors
	//---

	toBeCreatedServiceMonitors := []*monitoringv1.ServiceMonitor{}
	toBeUpdatedServiceMonitors := []*monitoringv1.ServiceMonitor{}
	toBeDeletedServiceMonitors := []*monitoringv1.ServiceMonitor{}

	for _, serviceMonitor := range serviceMonitorList.Items {
		found := false

		serviceMonitorName := types.NamespacedName{
			Name:      serviceMonitor.Name,
			Namespace: serviceMonitor.Namespace,
		}

		for _, meteredServiceMonitor := range meteredServiceMonitors.Items {
			name := meteredServiceMonitor.ObjectMeta.Labels["marketplace.redhat.com/serviceMonitor.Name"]
			namespace := meteredServiceMonitor.ObjectMeta.Labels["marketplace.redhat.com/serviceMonitor.Namespace"]
			foundName := types.NamespacedName{Name: name, Namespace: namespace}

			if foundName == serviceMonitorName {
				found = true
				toBeUpdatedServiceMonitors = append(toBeUpdatedServiceMonitors, meteredServiceMonitor)
				break
			}
		}

		if !found {
			toBeCreatedServiceMonitors = append(toBeCreatedServiceMonitors, serviceMonitor)
		}
	}

	// look for meteredServiceMonitors we've created by looking at labels
	for _, meteredServiceMonitor := range meteredServiceMonitors.Items {
		found := false
		name := meteredServiceMonitor.ObjectMeta.Labels["marketplace.redhat.com/serviceMonitor.Name"]
		namespace := meteredServiceMonitor.ObjectMeta.Labels["marketplace.redhat.com/serviceMonitor.Namespace"]
		foundName := types.NamespacedName{Name: name, Namespace: namespace}

		for _, serviceMonitor := range serviceMonitorList.Items {
			serviceMonitorName := types.NamespacedName{
				Name:      serviceMonitor.Name,
				Namespace: serviceMonitor.Namespace,
			}
			if foundName == serviceMonitorName {
				found = true
				break
			}
		}

		if !found {
			toBeDeletedServiceMonitors = append(toBeCreatedServiceMonitors, meteredServiceMonitor)
		}
	}

	//---
	// Reconcile Kube State Monitor for the def
	//---

	var kubeStateMonitor *monitoringv1.ServiceMonitor
	// if we have more than 1, we'll add the first and delete the rest
	if len(kubeStateServiceMonitors.Items) >= 1 {
		for idx, serviceMonitor := range kubeStateServiceMonitors.Items {
			if idx == 0 {
				kubeStateMonitor = kubeStateServiceMonitors.Items[0]

			} else {
				toBeDeletedServiceMonitors = append(toBeDeletedServiceMonitors, serviceMonitor)
			}
		}
	}

	//---
	// Logging our actions
	//---

	reqLogger.Info("finished calculating new state for service monitors",
		"toBeCreated", len(toBeCreatedServiceMonitors),
		"toBeUpdated", len(toBeUpdatedServiceMonitors),
		"toBeDeleted", len(toBeDeletedServiceMonitors))

	reqLogger.Info("finished kube state monitor check",
		"isNil", (kubeStateMonitor == nil))

	//---
	// Adjust our state
	//---

	instance.Status.Pods = []*metav1.ObjectMeta{}
	instance.Status.ServiceMonitors = []*metav1.ObjectMeta{}
	instance.Status.ServiceLabels = instance.Spec.ServiceMeterLabels
	instance.Status.PodLabels = instance.Spec.PodMeterLabels

	// best effort delete
	for _, serviceMonitor := range toBeDeletedServiceMonitors {
		if err := r.client.Delete(context.TODO(), serviceMonitor, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to delete service monitor", "serviceMonitor", serviceMonitor)
		} else {
			log.V(0).Info("deleted service monitor failed", "serviceMonitor", serviceMonitor)
		}
	}

	// create new service monitors
	for _, serviceMonitor := range toBeCreatedServiceMonitors {
		newMonitor := &monitoringv1.ServiceMonitor{}

		newMonitor.GenerateName = "rhm-metering-monitor-"
		newMonitor.Namespace = instance.Namespace
		newMonitor.Labels = labelsForServiceMonitor(serviceMonitor.Name, serviceMonitor.Namespace)
		newMonitor.Spec = serviceMonitor.Spec
		newMonitor.Spec.NamespaceSelector.MatchNames = []string{serviceMonitor.Namespace}
		configureServiceMonitorFromMeterLabels(instance, newMonitor)

		err := r.client.Create(context.TODO(), newMonitor)
		if err != nil {
			reqLogger.Error(err, "Failed to create service monitor on cluster")
			return reconcile.Result{}, err
		}

		if err := controllerutil.SetControllerReference(instance, newMonitor, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		reqLogger.Info("service monitor created successfully")
		instance.Status.ServiceMonitors = append(instance.Status.ServiceMonitors, &newMonitor.ObjectMeta)
	}

	if kubeStateMonitor == nil {
		kubeStateMonitor = openshiftKubeStateMonitor.DeepCopy()
		kubeStateMonitor.ObjectMeta = metav1.ObjectMeta{
			GenerateName: instance.Name,
		}
	}

	// update service monitor

	for _, serviceMonitor := range toBeUpdatedServiceMonitors {
		// TODO: add code to update
		instance.Status.ServiceMonitors = append(instance.Status.ServiceMonitors, &serviceMonitor.ObjectMeta)
	}

	//---
	// Save our state
	//---

	reqLogger.Info("updating state on meterdefinition")
	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update Prometheus status.")
		return reconcile.Result{}, err
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{}, nil
}

func (r *ReconcileMeterDefinition) finalizeMeterDefinition(req *marketplacev1alpha1.MeterDefinition) (reconcile.Result, error) {
	var err error

	// TODO: add finalizers

	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), meterDefinitionFinalizer))
	err = r.client.Update(context.TODO(), req)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// addFinalizer adds finalizers to the RazeeDeployment CR
func (r *ReconcileMeterDefinition) addFinalizer(instance *marketplacev1alpha1.MeterDefinition) error {
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

func configureServiceMonitorFromMeterLabels(def *marketplacev1alpha1.MeterDefinition, monitor *monitoringv1.ServiceMonitor) {
	endpoints := []monitoringv1.Endpoint{}
	for _, endpoint := range monitor.Spec.Endpoints {
		newEndpoint := endpoint.DeepCopy()
		relabelConfigs := []*monitoringv1.RelabelConfig{
			makeRelabelReplaceConfig([]string{"__name__"}, "meter_kind", "(.*)", def.Spec.MeterKind),
			makeRelabelReplaceConfig([]string{"__name__"}, "meter_domain", "(.*)", def.Spec.MeterDomain),
		}
		metricRelabelConfigs := []*monitoringv1.RelabelConfig{
			makeRelabelKeepConfig([]string{"__name__"}, labelsToRegex(def.Spec.ServiceMeterLabels)),
		}
		newEndpoint.RelabelConfigs = append(newEndpoint.RelabelConfigs, relabelConfigs...)
		newEndpoint.MetricRelabelConfigs = metricRelabelConfigs
		endpoints = append(endpoints, *newEndpoint)
	}
	monitor.Spec.Endpoints = endpoints
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
