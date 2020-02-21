package meterbase

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"

	"github.com/gotidy/ptr"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/marketplace-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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

var log = logf.Log.WithName("controller_meterbase")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner MeterBase
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
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
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
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

	// reconcile the base cfg
	foundcfg := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, foundcfg)
	if err != nil && errors.IsNotFound(err) {
		// Define a new configmap
		cfgBaseFileName := "assets/prometheus/base-configmap.yaml"
		basecfg, err := newBaseConfigMap(cfgBaseFileName, instance)

		if err != nil {
			reqLogger.Error(err, "Failed to create a new configmap because of file error.", "Configmap.Namespace", basecfg.Namespace, "Configmap.Name", basecfg.Name)
			return reconcile.Result{}, err
		}

		reqLogger.Info("Creating a new configmap.", "Configmap.Namespace", basecfg.Namespace, "Configmap.Name", basecfg.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create a new configmap.", "Configmap.Namespace", basecfg.Namespace, "Configmap.Name", basecfg.Name)
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get configmap.")
		return reconcile.Result{}, err
	}

	// replace with a config file load
	promOpts := &PromOpts{
		PullPolicy: "ifmissing",
		Images: Images{
			ConfigmapReload: "jimmidyson/configmap-reload:v0.3.0",
			Server:          "prom/prometheus:v2.15.2",
		},
	}

	statefulSet := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, statefulSet)
	if err != nil && errors.IsNotFound(err) {
		// Define a new statefulset
		dep := newPromDeploymentForCR(instance, promOpts)
		reqLogger.Info("Creating a new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new StatefulSet.", "Statefulset.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		// Deployment created successfully - return and requeue
		// NOTE: that the requeue is made with the purpose to provide the deployment object for the next step to ensure the deployment size is the same as the spec.
		// Also, you could GET the deployment object again instead of requeue if you wish. See more over it here: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment.")
		return reconcile.Result{}, err
	}

	// Set MeterBase instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, statefulSet, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Statefulset already exists - don't requeue
	reqLogger.Info("Skip reconcile: StatefulSet already exists", "Statefulset.Namespace", statefulSet.Namespace, "Statefulset.Name", statefulSet.Name)
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

// newPromDeployedForCR creates a statefulset for prometheus for our marketplace
// metering to use
func newPromDeploymentForCR(cr *marketplacev1alpha1.MeterBase, opt *PromOpts) *appsv1.StatefulSet {
	labels := map[string]string{
		"app": cr.Name,
	}

	metadata := metav1.ObjectMeta{
		Name:      cr.Name + "-statefulset",
		Namespace: cr.Namespace,
		Labels:    labels,
	}

	pvc := utils.NewPersistentVolumeClaim("storage-volume", &utils.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-prom-pvc",
			Namespace: cr.Namespace,
		},
		StorageClass: cr.Spec.Prometheus.Storage.Class,
		StorageSize:  &cr.Spec.Prometheus.Storage.Size,
	})

	var port int32 = 9090

	configMapName := cr.Name + "-configmap"

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
		Image:           opt.Images.Server,
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
		ReadinessProbe: makeProbe("/-/ready", port, 30, 30),
		LivenessProbe:  makeProbe("/-/healthy", port, 30, 30),
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

	return &appsv1.StatefulSet{
		ObjectMeta: metadata,
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.Int32(1),
			Selector: cr.Spec.Prometheus.Selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cr.Name + "-server-pod",
					Namespace: cr.Namespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						reloadContainer,
						serverContainer,
					},
					Volumes: []corev1.Volume{
						configVolume,
					},
				},
			},
			ServiceName: cr.Name + "-prom",
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				pvc,
			},
		},
	}
}

func newBaseConfigMap(filename string, cr *marketplacev1alpha1.MeterBase) (*corev1.ConfigMap, error) {
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
	cfg.Name = cr.Name + "-base-cfg"

	return cfg, nil
}

// makeProbe creates a probe with the specified path and prot
func makeProbe(path string, port, initialDelaySeconds, timeoutSeconds int32) *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt(int(port)),
			},
		},
		InitialDelaySeconds: initialDelaySeconds,
		TimeoutSeconds:      timeoutSeconds,
	}
}
