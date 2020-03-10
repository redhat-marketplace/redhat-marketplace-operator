package meterbase

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"

	"github.com/gotidy/ptr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/marketplace-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8yaml "k8s.io/apimachinery/pkg/util/yaml"
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
	return &ReconcileMeterBase{client: mgr.GetClient(), scheme: mgr.GetScheme()}
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

	// watch statefulset
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
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

	// reconcile the base cfg
	foundcfg := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, foundcfg)
	if err != nil && errors.IsNotFound(err) {
		// Define a new configmap
		assetBase := viper.GetString("assets")
		cfgBaseFileName := filepath.Join(assetBase, "prometheus/base-configmap.yaml")
		reqLogger.Info("looking up configmap at", "assetBase", assetBase)
		basecfg, err := r.newBaseConfigMap(cfgBaseFileName, instance)

		if err != nil {
			reqLogger.Error(err, "Failed to create a new configmap because of file error.")
			return reconcile.Result{}, err
		}

		reqLogger.Info("Creating a new configmap.", "Configmap.Namespace", basecfg.Namespace, "Configmap.Name", basecfg.Name)
		err = r.client.Create(context.TODO(), basecfg)
		if err != nil {
			reqLogger.Error(err, "Failed to create a new configmap.", "Configmap.Namespace", basecfg.Namespace, "Configmap.Name", basecfg.Name)
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get configmap.")
		return reconcile.Result{}, err
	}

	// Set MeterBase instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, foundcfg, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// replace with a config file load
	promOpts := &PromOpts{
		PullPolicy: "IfNotPresent",
		Images: Images{
			Server:          viper.GetString("related-image-prom-server"),
			ConfigmapReload: viper.GetString("related-image-configmap-reload"),
		},
	}

	statefulSet := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, statefulSet)
	if err != nil && errors.IsNotFound(err) {
		// Define a new statefulset
		dep, err := r.newPromStatefulsetForCR(instance, promOpts)

		if err != nil {
			reqLogger.Error(err, "Failed to create new StatefulSet.")
			return reconcile.Result{}, err
		}

		reqLogger.Info("Creating a new StatefulSet.", "StatefulSet.Namespace", dep.Namespace, "StatefulSet.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new StatefulSet.", "Statefulset.Namespace", dep.Namespace, "StatefulSet.Name", dep.Name)
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment.")
		return reconcile.Result{}, err
	}

	// Set MeterBase instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, statefulSet, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	updateStatefulset := statefulSet.DeepCopy()

	if !reflect.DeepEqual(
		statefulSet.Spec.Template.Spec.NodeSelector,
		instance.Spec.Prometheus.NodeSelector) {
		reqLogger.Info("Detected a change in node selector")
		updateStatefulset.Spec.Template.Spec.NodeSelector = instance.Spec.Prometheus.NodeSelector
	}

	for _, container := range updateStatefulset.Spec.Template.Spec.Containers {
		if container.Name == instance.Name+"-server" {
			if !reflect.DeepEqual(container.Resources, instance.Spec.Prometheus.ResourceRequirements) {
				reqLogger.Info("Detected a change in resource requirements")
				container.Resources = instance.Spec.Prometheus.ResourceRequirements
			}
		}
	}

	if !reflect.DeepEqual(updateStatefulset, statefulSet) {
		reqLogger.Info("Updating the statefulset.", "StatefulSet.Namespace", updateStatefulset.Namespace, "StatefulSet.Name", updateStatefulset.Name)
		err = r.client.Update(context.TODO(), updateStatefulset)
		if err != nil {
			reqLogger.Error(err, "Failed to update StatefulSet.", "Statefulset.Namespace", updateStatefulset.Namespace, "StatefulSet.Name", updateStatefulset.Name)
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	service := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		// Define a new statefulset
		newService := r.serviceForPrometheus(instance)
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

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelsForPrometheus(instance.Name)),
	}
	err = r.client.List(context.TODO(), podList, listOpts...)
	if err != nil {
		reqLogger.Error(err, "Failed to list pods.",
			"MeterBase.Namespace", instance.Namespace,
			"MeterBase.Name", instance.Name)
		return reconcile.Result{}, err
	}

	podNames := utils.GetPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, instance.Status.PrometheusNodes) {
		instance.Status.PrometheusNodes = podNames
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update Prometheus status.")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// jimmidyson/configmap-reload:v0.3.0
// prom/prometheus:v2.15.2

// configPath: /etc/config/prometheus.yml

type Images struct {
	ConfigmapReload string
	Server          string
}

type PromOpts struct {
	corev1.PullPolicy
	Images
}

// newPromStatefulsetForCR creates a statefulset for prometheus for our marketplace
// metering to use
func (r *ReconcileMeterBase) newPromStatefulsetForCR(cr *marketplacev1alpha1.MeterBase, opt *PromOpts) (*appsv1.StatefulSet, error) {
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
		},
		StorageClass: &storageClass,
		StorageSize:  &cr.Spec.Prometheus.Storage.Size,
	})

	if err != nil {
		return nil, err
	}

	var port int32 = 9090

	configMapName := cr.Name

	configVolumeMount := corev1.VolumeMount{
		Name:      "config-volume",
		MountPath: "/etc/config",
	}

	storageVolumeMount := corev1.VolumeMount{
		Name:      "storage-volume",
		MountPath: "/data",
	}

	configFile := fmt.Sprintf("%v/prometheus.yml", configVolumeMount.MountPath)
	retentionTime := "15d"

	reloadContainer := corev1.Container{
		Name:            cr.Name + "-configmap-reload",
		ImagePullPolicy: opt.PullPolicy,
		Image:           opt.Images.ConfigmapReload,
		Args: []string{
			fmt.Sprintf("--volume-dir=%v", configVolumeMount.MountPath),
			fmt.Sprintf("--webhook-url=http://127.0.0.1:%v/-/reload", port),
		},
		VolumeMounts: []corev1.VolumeMount{
			configVolumeMount,
		},
	}

	serverContainer := corev1.Container{
		Name:            cr.Name + "-server",
		ImagePullPolicy: opt.PullPolicy,
		Image:           opt.Images.Server,
		Args: []string{
			fmt.Sprintf("--config.file=%v", configFile),
			fmt.Sprintf("--storage.tsdb.retention.time=%v", retentionTime),
			fmt.Sprintf("--storage.tsdb.path=%v", storageVolumeMount.Name),
		},
		VolumeMounts: []corev1.VolumeMount{
			configVolumeMount,
			storageVolumeMount,
		},
		Ports: []corev1.ContainerPort{
			corev1.ContainerPort{
				ContainerPort: port,
			},
		},
		ReadinessProbe: utils.MakeProbe("/-/ready", port, 30, 30),
		LivenessProbe:  utils.MakeProbe("/-/healthy", port, 30, 30),
		Resources:      cr.Spec.Prometheus.ResourceRequirements,
	}

	configVolume := corev1.Volume{
		Name: "config-volume",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
			},
		},
	}

	nodeSelector := map[string]string{}

	if cr.Spec.Prometheus.NodeSelector != nil {
		nodeSelector = cr.Spec.Prometheus.NodeSelector
	}

	stf := &appsv1.StatefulSet{
		ObjectMeta: metadata,
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.Int32(1),
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cr.Name + "-server-pod",
					Namespace: cr.Namespace,
					Labels:    ls,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "marketplace-operator",
					Containers: []corev1.Container{
						reloadContainer,
						serverContainer,
					},
					Volumes: []corev1.Volume{
						configVolume,
					},
					NodeSelector: nodeSelector,
				},
			},
			ServiceName: cr.Name + "-prom",
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				pvc,
			},
		},
	}

	return stf, nil
}

// serviceForPrometheus function takes in a Prometheus object and returns a Service for that object.
func (r *ReconcileMeterBase) serviceForPrometheus(cr *marketplacev1alpha1.MeterBase) *corev1.Service {
	var port int32 = 9090
	ls := labelsForPrometheus(cr.Name)

	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{
				{
					Port: port,
					Name: cr.Name,
				},
			},
			ClusterIP: "None",
		},
	}
	return ser
}

// func loadYAML(x reflect.Type) interface{} {
// 	dec := k8yaml.NewYAMLOrJSONDecoder(bytes.NewReader(dat), 1000)
// 	var genericTypeVal interface{}
// 	switch x {
// 	case type(corev1.ConfigMap):
// 		genericTypeVal = &corev1.ConfigMap{}
// 	}

// 	if err := dec.Decode(&genericTypeVal); err != nil {
// 			return nil, err
// 		}

// 		return cfg
// }

// func useX() {
// 	value1 := genericFunctionX()

// 	if ok, cfg := value1.(corev1.Configmap);  !ok {
// 		return nil
// 	}

// 	return cfg
//}

func (r *ReconcileMeterBase) newBaseConfigMap(filename string, cr *marketplacev1alpha1.MeterBase) (*corev1.ConfigMap, error) {
	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	cfg := &corev1.ConfigMap{}
	dec := k8yaml.NewYAMLOrJSONDecoder(bytes.NewReader(dat), 1000)

	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}

	cfg.Namespace = cr.Namespace
	cfg.Name = cr.Name

	return cfg, nil
}

// labelsForPrometheus returns the labels for selecting the resources
// belonging to the given prometheus CR name.
func labelsForPrometheus(name string) map[string]string {
	return map[string]string{"app": "meterbase-prom", "meterbase_cr": name}
}
