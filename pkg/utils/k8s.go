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
	StorageSize  *resource.Quantity
	AccessMode   *corev1.PersistentVolumeAccessMode
}

func NewPersistentVolumeClaim(volumeName string, pv *PersistentVolume) corev1.PersistentVolumeClaim {
	// set some defaults
	quantity := resource.MustParse("20Gi")
	accessMode := corev1.PersistentVolumeAccessMode("ReadWriteOnce")
	values := &PersistentVolume{
		StorageClass: ptr.String(""),
		AccessMode:   &accessMode,
		StorageSize:  &quantity,
	}

	// merge values from pv into values
	mergo.Merge(&values, pv)

	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        values.Name,
			Annotations: values.Annotations,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: volumeName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				*values.AccessMode,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": quantity,
				},
			},
			StorageClassName: values.StorageClass,
		},
	}
}
