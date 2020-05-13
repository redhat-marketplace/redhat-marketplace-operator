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

package meterbase

import (
	"context"
	"path/filepath"
	"reflect"
	"time"

	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/operator-framework/api/pkg/operators"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	DEFAULT_PROM_SERVER            = "prom/prometheus:v2.15.2"
	DEFAULT_CONFIGMAP_RELOAD       = "jimmidyson/configmap-reload:v0.3.0"
	RELATED_IMAGE_PROM_SERVER      = "RELATED_IMAGE_PROM_SERVER"
	RELATED_IMAGE_CONFIGMAP_RELOAD = "RELATED_IMAGE_CONFIGMAP_RELOAD"
)

//ConfigmapReload: "jimmidyson/configmap-reload:v0.3.0",
//Server:          "prom/prometheus:v2.15.2",

var (
	log = logf.Log.WithName("marketplace_op_controller_meterbase")

	meterbaseFlagSet *pflag.FlagSet
)

func init() {
	meterbaseFlagSet = pflag.NewFlagSet("meterbase", pflag.ExitOnError)
	meterbaseFlagSet.String("related-image-prom-server",
		utils.Getenv(RELATED_IMAGE_PROM_SERVER, DEFAULT_PROM_SERVER),
		"image for prometheus")
	meterbaseFlagSet.String("related-image-configmap-reload",
		utils.Getenv(RELATED_IMAGE_CONFIGMAP_RELOAD, DEFAULT_CONFIGMAP_RELOAD),
		"image for prometheus")
}

func FlagSet() *pflag.FlagSet {
	return meterbaseFlagSet
}

// Add creates a new MeterBase Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	promOpts := &MeterbaseOpts{
		PullPolicy: "IfNotPresent",
		AssetPath:  viper.GetString("assets"),
	}
	return &ReconcileMeterBase{client: mgr.GetClient(), scheme: mgr.GetScheme(), opts: promOpts}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("meterbase-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MeterBase
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MeterBase{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// watch configmap
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	// watch prometheus
	err = c.Watch(&source.Kind{Type: &monitoringv1.Prometheus{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	// watch headless service
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMeterBase implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMeterBase{}

// ReconcileMeterBase reconciles a MeterBase object
type ReconcileMeterBase struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	opts   *MeterbaseOpts
}

// Reconcile reads that state of the cluster for a MeterBase object and makes changes based on the state read
// and what is in the MeterBase.Spec
func (r *ReconcileMeterBase) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterBase")
	cc := NewClientCommand(r.client, r.scheme, reqLogger)

	// Fetch the MeterBase instance
	instance := &marketplacev1alpha1.MeterBase{}
	result, reconcileResult, err := cc.Execute([]ClientAction{
		GetAction{
			NamespacedName: request.NamespacedName,
			Object:         instance,
		},
	})

	if result == Error {
		reqLogger.Error(err, "Failed to get MeterBase.")
		return reconcileResult, err
	}

	if result == NotFound {
		reqLogger.Info("MeterBase resource not found. Ignoring since object must be deleted.")
		return reconcile.Result{}, nil
	}

	// if instance.Enabled == false
	// return do nothing
	if !instance.Spec.Enabled {
		reqLogger.Info("MeterBase resource found but ignoring since metering is not enabled.")
		return reconcile.Result{}, nil
	}

	prometheus := &monitoringv1.Prometheus{}
	result, reconcileResult, err = cc.Execute([]ClientAction{
		GetAction{
			NamespacedName: types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
			Object:         prometheus,
		},
		FilterAction{
			Options: []FilterActionOption{
				FilterByActionResult(func(result ActionResult) bool {
					return result == NotFound
				}),
			},
			Action: CreateAction{
				NewObject: func() (runtime.Object, error) {
					return r.newPrometheusOperator(instance, r.opts)
				},
				Options: []CreateActionOption{
					CreateWithAddOwner(instance),
					CreateWithPatch(true),
				},
			},
		},
	})

	if result == Requeue {
		return reconcileResult, err
	}

	service := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		// Define a new statefulset
		newService := r.serviceForPrometheus(instance, 9090)
		reqLogger.Info("Creating a new Service.", "Service.Namespace", newService.Namespace, "Service.Name", newService.Name)
		err = r.client.Create(context.TODO(), newService)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Service.", "Service.Namespace", newService.Namespace, "Service.Name", newService.Name)
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service.")
		return reconcile.Result{}, err
	}

	// Set MeterBase instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, service, r.scheme); err != nil {
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

	// find specific service monitor for kubelet
	openshiftKubeletMonitor := &monitoringv1.ServiceMonitor{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: "openshift-monitoring",
		Name:      "kubelet",
	}, openshiftKubeletMonitor)
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		reqLogger.Info("can't find openshift kube state")
	}

	//---
	// Reconcile Kube State Monitor for the def
	//---

	meteringKubeStateMonitor := &monitoringv1.ServiceMonitor{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      "kube-state-metrics",
	}, meteringKubeStateMonitor)
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		reqLogger.Info("can't find openshift kube state")
	}

	meteringKubeStateMonitorNotFound := errors.IsNotFound(err)

	//---
	// Reconcile kubelet monitor for the def
	//---

	meteringKubeletMonitor := &monitoringv1.ServiceMonitor{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      "kubelet",
	}, meteringKubeletMonitor)

	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		reqLogger.Info("can't find openshift kube state")
	}

	meteringKubeletMonitorNotFound := errors.IsNotFound(err)

	reqLogger.Info("finished kube state monitor check", "isNotFound", meteringKubeStateMonitorNotFound)
	reqLogger.Info("finished kubelet monitor check", "isNotFound", meteringKubeletMonitorNotFound)

	// Create kube state
	newKubeState := &monitoringv1.ServiceMonitor{}
	newKubeState.Name = openshiftKubeletMonitor.Name
	newKubeState.Namespace = instance.Namespace
	newKubeState.Name = openshiftKubeStateMonitor.Name
	newKubeState.Spec = *openshiftKubeStateMonitor.Spec.DeepCopy()
	newKubeState.Labels = labelsForServiceMonitor(openshiftKubeStateMonitor.Name, openshiftKubeStateMonitor.Namespace)
	newKubeState.Spec.NamespaceSelector.MatchNames = []string{openshiftKubeStateMonitor.Namespace}

	if meteringKubeStateMonitorNotFound {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newKubeState); err != nil {
			reqLogger.Error(err, "Failed to set annotation")
			return reconcile.Result{}, err
		}

		err := r.client.Create(context.TODO(), newKubeState)

		if err != nil {
			reqLogger.Error(err, "failed to create new kube state service monitor")
			return reconcile.Result{}, err
		}

		reqLogger.Info("created the kube state monitor")
		return reconcile.Result{Requeue: true}, nil
	}

	// Create kubelet
	newKubelet := &monitoringv1.ServiceMonitor{}
	newKubelet.Name = openshiftKubeletMonitor.Name
	newKubelet.Namespace = instance.Namespace
	newKubelet.Spec = *openshiftKubeletMonitor.Spec.DeepCopy()
	newKubelet.Labels = labelsForServiceMonitor(openshiftKubeletMonitor.Name, openshiftKubeletMonitor.Namespace)
	newKubelet.Spec.NamespaceSelector.MatchNames = []string{openshiftKubeletMonitor.Namespace}

	if meteringKubeletMonitorNotFound {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newKubelet); err != nil {
			reqLogger.Error(err, "Failed to set annotation")
			return reconcile.Result{}, err
		}

		err := r.client.Create(context.TODO(), newKubelet)

		if err != nil {
			reqLogger.Error(err, "failed to create new kubelet service monitor")
			return reconcile.Result{}, err
		}

		reqLogger.Info("created the kubelet monitor")
		return reconcile.Result{Requeue: true}, nil
	}

	// ----
	// Check if prometheus needs updating
	// ----

	expectedPrometheusSpec, err := r.newPrometheusOperator(instance, r.opts)

	if err != nil {
		reqLogger.Error(err, "Failed to get prometheus spec.")
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(prometheus.Spec.ServiceMonitorNamespaceSelector, expectedPrometheusSpec.Spec.ServiceMonitorNamespaceSelector) {
		reqLogger.Info("updating service monitor namespace selector")
		prometheus.Spec.ServiceMonitorNamespaceSelector = expectedPrometheusSpec.Spec.ServiceMonitorNamespaceSelector

		err := r.client.Update(context.TODO(), prometheus)

		if err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, err
	}

	// ----
	// Update our status
	// ----

	if instance.Status.PrometheusStatus == nil ||
		!reflect.DeepEqual(instance.Status.PrometheusStatus, prometheus.Status) {
		reqLogger.Info("updating prometheus status")
		instance.Status.PrometheusStatus = prometheus.Status
		err := r.client.Status().Update(context.TODO(), instance)

		if err != nil {
			reqLogger.Error(err, "Failed to update Prometheus status.")
			return reconcile.Result{}, err
		}
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{}, nil
}

// jimmidyson/configmap-reload:v0.3.0
// prom/prometheus:v2.15.2

// configPath: /etc/config/prometheus.yml

type Images struct {
	ConfigmapReload string
	Server          string
}

type MeterbaseOpts struct {
	corev1.PullPolicy
	AssetPath string
}

func (r *ReconcileMeterBase) newPrometheusOperator(cr *marketplacev1alpha1.MeterBase, opt *MeterbaseOpts) (*monitoringv1.Prometheus, error) {
	ls := labelsForPrometheus(cr.Name)

	metadata := metav1.ObjectMeta{
		Name:      cr.Name,
		Namespace: cr.Namespace,
		Labels:    ls,
	}

	storageClass := ""
	if cr.Spec.Prometheus.Storage.Class == nil {
		foundDefaultClass, err := utils.GetDefaultStorageClass(r.client)

		if err != nil {
			log.Error(err, "no default class found")
		} else {
			storageClass = foundDefaultClass
		}
	} else {
		storageClass = *cr.Spec.Prometheus.Storage.Class
	}

	pvc, err := utils.NewPersistentVolumeClaim(utils.PersistentVolume{
		ObjectMeta: &metav1.ObjectMeta{
			Name: "storage-volume",
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
		},
		StorageClass: &storageClass,
		StorageSize:  &cr.Spec.Prometheus.Storage.Size,
	})

	if err != nil {
		return nil, err
	}

	nodeSelector := map[string]string{}

	if cr.Spec.Prometheus.NodeSelector != nil {
		nodeSelector = cr.Spec.Prometheus.NodeSelector
	}

	assetBase := opt.AssetPath
	cfgBaseFileName := filepath.Join(assetBase, "prometheus/prometheus.yaml")
	prom, err := r.newPrometheus(cfgBaseFileName, cr)

	if err != nil {
		return nil, err
	}

	prom.ObjectMeta = metadata
	prom.Spec.NodeSelector = nodeSelector
	prom.Spec.Storage.VolumeClaimTemplate = pvc
	prom.Spec.Resources = cr.Spec.Prometheus.ResourceRequirements

	return prom, nil
}

// serviceForPrometheus function takes in a Prometheus object and returns a Service for that object.
func (r *ReconcileMeterBase) serviceForPrometheus(
	cr *marketplacev1alpha1.MeterBase,
	port int32) *corev1.Service {

	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"prometheus": cr.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "web",
					Port:       port,
					TargetPort: intstr.FromString("web"),
				},
			},
			ClusterIP: "None",
		},
	}
	return ser
}

func (r *ReconcileMeterBase) newBaseConfigMap(filename string, cr *marketplacev1alpha1.MeterBase) (*corev1.ConfigMap, error) {
	int, err := utils.LoadYAML(filename, corev1.ConfigMap{})
	if err != nil {
		return nil, err
	}

	cfg := (int).(*corev1.ConfigMap)
	cfg.Namespace = cr.Namespace
	cfg.Name = cr.Name

	return cfg, nil
}

func (r *ReconcileMeterBase) newPrometheus(filename string, cr *marketplacev1alpha1.MeterBase) (*monitoringv1.Prometheus, error) {
	int, err := utils.LoadYAML(filename, monitoringv1.Prometheus{})
	if err != nil {
		return nil, err
	}

	prom := (int).(*monitoringv1.Prometheus)
	prom.Namespace = cr.Namespace
	prom.Name = cr.Name

	return prom, nil
}

func (r *ReconcileMeterBase) newPrometheusSubscription(
	instance *marketplacev1alpha1.MeterBase,
	labels map[string]string,
) *operators.Subscription {
	return &operators.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus",
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: &operators.SubscriptionSpec{
			Channel:                "beta",
			InstallPlanApproval:    "Automatic",
			Package:                "prometheus",
			CatalogSource:          "community-operators",
			CatalogSourceNamespace: "openshift-marketplace",
		},
	}
}

// labelsForPrometheus returns the labels for selecting the resources
// belonging to the given prometheus CR name.
func labelsForPrometheus(name string) map[string]string {
	return map[string]string{"app": "meterbase-prom", "meterbase_cr": name}
}

// labelsForPrometheusOperator returns the labels for selecting the resources
// belonging to the given prometheus CR name.
func labelsForPrometheusOperator(name string) map[string]string {
	return map[string]string{"prometheus": name}
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
