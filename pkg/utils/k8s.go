package utils

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/gotidy/ptr"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8yaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
)

type PersistentVolume struct {
	*metav1.ObjectMeta
	StorageClass *string
	StorageSize  *resource.Quantity
	AccessMode   *corev1.PersistentVolumeAccessMode
}

func NewPersistentVolumeClaim(values PersistentVolume) (corev1.PersistentVolumeClaim, error) {
	// set some defaults
	quantity := resource.MustParse("20Gi")
	accessMode := corev1.ReadWriteOnce
	defaults := PersistentVolume{
		ObjectMeta:   &metav1.ObjectMeta{},
		StorageClass: ptr.String(""),
		AccessMode:   &accessMode,
		StorageSize:  &quantity,
	}

	// merge values from pv into values
	if err := mergo.Merge(&values, defaults); err != nil {
		return corev1.PersistentVolumeClaim{}, err
	}

	return corev1.PersistentVolumeClaim{
		ObjectMeta: *values.ObjectMeta,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				*values.AccessMode,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": *values.StorageSize,
				},
			},
			StorageClassName: values.StorageClass,
		},
	}, nil
}

// GetPodNames returns the pod names of the array of pods passed in
func GetPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// GetDefaultStorageClass attempts to return the default storage class
// of the cluster and errors if it cannot be found
func GetDefaultStorageClass(client client.Client) (string, error) {
	storageList := &storagev1.StorageClassList{}

	if err := client.List(context.TODO(), storageList); err != nil {
		return "", err
	}

	defaultStorageOptions := []string{}

	for _, storageClass := range storageList.Items {
		if storageClass.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			defaultStorageOptions = append(defaultStorageOptions, storageClass.Name)
		}
	}

	if len(defaultStorageOptions) == 0 {
		return "", fmt.Errorf("could not find a default storage class")
	}

	if len(defaultStorageOptions) > 1 {
		return "", fmt.Errorf("multiple default options, cannot pick one")
	}

	return defaultStorageOptions[0], nil
}

// MakeProbe creates a probe with the specified path and prot
func MakeProbe(path string, port, initialDelaySeconds, timeoutSeconds int32) *corev1.Probe {
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

// BuildNewOpSrc returns a new Operator Source
func BuildNewOpSrc() *opsrcv1.OperatorSource {
	opsrc := &opsrcv1.OperatorSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: OPSRC_NAME,
			// Must always be openshift-marketplace
			Namespace: OPERATOR_MKTPLACE_NS,
		},
		Spec: opsrcv1.OperatorSourceSpec{
			DisplayName:       "Red Hat Marketplace",
			Endpoint:          "https://quay.io/cnr",
			Publisher:         "Red Hat Marketplace",
			RegistryNamespace: "redhat-marketplace",
			Type:              "appregistry",
		},
	}

	return opsrc
}

// BuildRazeeCrd returns a RazeeDeployment cr with default values
func BuildRazeeCr(namespace, clusterUUID string, deploySecretName *string) *marketplacev1alpha1.RazeeDeployment {

	cr := &marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RAZEE_NAME,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled:          true,
			ClusterUUID:      clusterUUID,
			DeploySecretName: deploySecretName,
		},
	}

	return cr
}

// BuildMeterBaseCr returns a MeterBase cr with default values
func BuildMeterBaseCr(namespace string) *marketplacev1alpha1.MeterBase {

	cr := &marketplacev1alpha1.MeterBase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      METERBASE_NAME,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MeterBaseSpec{
			Enabled: true,
			Prometheus: &marketplacev1alpha1.PrometheusSpec{
				Storage: marketplacev1alpha1.StorageSpec{
					Size: resource.MustParse("20Gi"),
				},
			},
		},
	}
	return cr
}

func LoadYAML(filename string, i interface{}) (interface{}, error) {
	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	dec := k8yaml.NewYAMLOrJSONDecoder(bytes.NewReader(dat), 1000)
	var genericTypeVal interface{}

	switch v := i.(type) {
	case corev1.ConfigMap:
		genericTypeVal = &corev1.ConfigMap{}
	case monitoringv1.Prometheus:
		genericTypeVal = &monitoringv1.Prometheus{}
	default:
		return nil, fmt.Errorf("type not recognized %T", v)
	}

	if err := dec.Decode(&genericTypeVal); err != nil {
		return nil, err
	}

	return genericTypeVal, nil
}

// ******************************************
// Gets passed a list of namespaces
// ******************************************
// Case 1: empty list: return all resources from ALL namesapces
// ******************************************
// Case 2: sinlge item in list: return all resources for that 1 namespace
// ******************************************
// Case 3: multiple items in list: return a list of all the resources in those namespaces
// to simplify for each item in list, call case 2
func filterByNamespace(namespaces []corev1.Namespace, resources corev1.ResourceList, rClient client.Client) (error, corev1.ResourceList) {
	var err error
	if namespaces == nil {
		//list options -> namespaces = blank -> get across all namespaces
		//list.get --> format
		listOpts := []client.ListOption{
			client.InNamespace(""),
		}
		err, resources = getResources(listOpts, resources, rClient)

	} else if len(namespaces) == 1 {
		//list options -> namespaces = namespaces -> get across that namespace
		listOpts := []client.ListOption{
			client.InNamespace(namespaces[0].Namespace),
		}
		err, resources = getResources(listOpts, resources, rClient)

	} else if len(namespaces) > 1 {
		//for each ns in namespaces
		//recursive call to resource by namespace (ns)
		//return all options?
		for _, ns := range namespaces {
			listOpts := []client.ListOption{
				client.InNamespace(ns.GetNamespace()),
			}
			err, resources = getResources(listOpts, resources, rClient)
		}
	}
	return err, resources
}

func getResources(listOpts []client.ListOption, resources corev1.ResourceList, rClient client.Client) (error, corev1.ResourceList) {
	var err error
	if reflect.TypeOf(resources) == reflect.TypeOf(corev1.ResourceName("Pod")) {
		podList := &corev1.PodList{}
		err = rClient.List(context.TODO(), podList, listOpts...)
		for _, pod := range podList.Items {
			resources[corev1.ResourceName(pod.GetName())] = resource.MustParse("1")
		}
	} else if reflect.TypeOf(resources) == reflect.TypeOf(corev1.ResourceName("ServiceMontor")) {
		serviceMonitorList := &monitoringv1.ServiceMonitorList{}
		err = rClient.List(context.TODO(), serviceMonitorList, listOpts...)
		for _, serviceMonitor := range serviceMonitorList.Items {
			resources[corev1.ResourceName(serviceMonitor.GetName())] = resource.MustParse("1")
		}
	} else {
		fmt.Println("------------ type of resource", reflect.TypeOf(resources))
	}

	return err, resources
}
