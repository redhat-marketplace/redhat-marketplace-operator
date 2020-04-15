package meterbase

import (
	"context"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"path/filepath"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
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

	// Fetch the MeterBase instance
	instance := &marketplacev1alpha1.MeterBase{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("MeterBase resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get MeterBase.")
		return reconcile.Result{}, err
	}

	// if instance.Enabled == false
	// return do nothing
	if !instance.Spec.Enabled {
		reqLogger.Info("MeterBase resource found but ignoring since metering is not enabled.")
		return reconcile.Result{}, nil
	}

	prometheus := &monitoringv1.Prometheus{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, prometheus)
	if err != nil && errors.IsNotFound(err) {
		// Define a new statefulset
		dep, err := r.newPrometheusOperator(instance, r.opts)

		if err != nil {
			reqLogger.Error(err, "Failed to create new Prometheus.")
			return reconcile.Result{}, err
		}

		reqLogger.Info("Creating a new Prometheus.", "Prometheus.Namespace", dep.Namespace, "Prometheus.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Prometheus.", "Statefulset.Namespace", dep.Namespace, "Prometheus.Name", dep.Name)
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Prometheus.")
		return reconcile.Result{}, err
	}

	// Set MeterBase instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, prometheus, r.scheme); err != nil {
		return reconcile.Result{}, err
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
