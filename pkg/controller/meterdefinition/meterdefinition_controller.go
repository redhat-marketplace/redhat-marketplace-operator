package meterdefinition

import (
	"context"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var log = logf.Log.WithName("controller_meterdefinition")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new MeterDefinition Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMeterDefinition{client: mgr.GetClient(), scheme: mgr.GetScheme()}
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

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner MeterDefinition
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
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
}

// Reconcile reads that state of the cluster for a MeterDefinition object and makes changes based on the state read
// and what is in the MeterDefinition.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMeterDefinition) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterDefinition")

	// Fetch the MeterDefinition instance
	instance := &marketplacev1alpha1.MeterDefinition{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// ---
	// Collect current state
	// ---
	serviceMonitorList := &monitoringv1.ServiceMonitorList{}
	listOpts := []client.ListOption{
		client.MatchingFields(instance.Spec.ServiceMonitorNamespaceSelector.MatchLabels),
		client.MatchingLabels(instance.Spec.ServiceMonitorSelector.MatchLabels),
	}
	err = r.client.List(context.TODO(), serviceMonitorList, listOpts...)

	if err != nil {
		reqLogger.Error(err, "Failed to list service monitors.",
			"MeterBase.Namespace", instance.Namespace,
			"MeterBase.Name", instance.Name)
		return reconcile.Result{}, err
	}

	podList := &corev1.PodList{}
	listOpts = []client.ListOption{
		client.MatchingFields(instance.Spec.PodNamespaceSelector.MatchLabels),
		client.MatchingLabels(instance.Spec.PodSelector.MatchLabels),
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
			"marketplace.redhat.com/metered": "true",
			"marketplace.redhat.com/kind":    "ServiceMonitor",
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
			"marketplace.redhat.com/metered": "true",
			"marketplace.redhat.com/kind":    "Pod",
		}),
		client.InNamespace(instance.Namespace),
	}
	err = r.client.List(context.TODO(), meteredPodList, listOpts...)

	if err != nil {
		reqLogger.Error(err, "Failed to list posd.",
			"MeterBase.Namespace", instance.Namespace,
			"MeterBase.Name", instance.Name)
		return reconcile.Result{}, err
	}

	// find specific service monitor for kube-state

	kubeStateServiceMonitors := &monitoringv1.ServiceMonitorList{}
	listOpts = []client.ListOption{
		client.MatchingLabels(map[string]string{
			"marketplace.redhat.com/metered":                   "true",
			"marketplace.redhat.com/kind":                      "ServiceMonitor",
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
		for idx, serviceMonitor := range kubeStateServiceMonitors {
			if idx == 0 {
				kubeStateMonitor = kubeStateServiceMonitors.Items[0]

			} else {
				toBeDeletedServiceMonitors = append(toBeDeletedServiceMonitors, serviceMonitor)
			}
		}
	}

	//---
	// Adjust our state
	//---

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
		newMonitor := serviceMonitor.DeepCopy()

		newMonitor.Name = ""
		newMonitor.GenerateName = "rhm-metering-monitor"
		newMonitor.ObjectMeta.Labels = labelsForServiceMonitor(serviceMonitor.Name, serviceMonitor.Namespace)

		instance.Status.ServiceMonitors = append(instance.Status.ServiceMonitors, newMonitor)
	}

	podMonitor := []*monitoringv1.ServiceMonitor{}
	toBeUpdatedServiceMonitors := []*monitoringv1.ServiceMonitor{}
	toBeDeletedServiceMonitors := []*monitoringv1.ServiceMonitor{}

	//---
	// Save our state
	//---

	return reconcile.Result{}, nil
}

func labelsForServiceMonitor(name, namespace string) map[string]string {
	return map[string]string{
		"marketplace.redhat.com/metered":                  "true",
		"marketplace.redhat.com/deployed":                  "true",
		"marketplace.redhat.com/serviceMonitor.Name":      name,
		"marketplace.redhat.com/serviceMonitor.Namespace": namespace,
	}
}
