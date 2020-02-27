package utils

import (
	"context"
	"fmt"

	"github.com/gotidy/ptr"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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


