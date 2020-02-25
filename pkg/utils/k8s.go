package utils

import (
	"github.com/gotidy/ptr"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PersistentVolume struct {
	metav1.ObjectMeta
	StorageClass *string
	StorageSize  resource.Quantity
	AccessMode   corev1.PersistentVolumeAccessMode
}

func NewPersistentVolumeClaim(volumeName string, values *PersistentVolume) corev1.PersistentVolumeClaim {
	// set some defaults
	quantity := resource.MustParse("20Gi")
	accessMode := corev1.PersistentVolumeAccessMode("ReadWriteOnce")
	defaults := &PersistentVolume{
		StorageClass: ptr.String(""),
		AccessMode:   accessMode,
		StorageSize:  quantity,
	}

	// merge values from pv into values
	mergo.Merge(&values, defaults, mergo.WithOverride)

	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        values.Name,
			Annotations: values.Annotations,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: volumeName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				values.AccessMode,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": values.StorageSize,
				},
			},
			StorageClassName: values.StorageClass,
		},
	}
}

// getPodNames returns the pod names of the array of pods passed in
func GetPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
