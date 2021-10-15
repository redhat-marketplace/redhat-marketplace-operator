package types

import (
	osimagev1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ImageSecretList struct {
	corev1.SecretList
}

func RegisterImageStream(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(osimagev1.GroupVersion,
		&osimagev1.Image{},
		&osimagev1.ImageList{},
		&osimagev1.ImageSignature{},
		&osimagev1.ImageStream{},
		&osimagev1.ImageStreamList{},
		&osimagev1.ImageStreamMapping{},
		&osimagev1.ImageStreamTag{},
		&osimagev1.ImageStreamTagList{},
		&osimagev1.ImageStreamImage{},
		&osimagev1.ImageStreamLayers{},
		&osimagev1.ImageStreamImport{},
		&osimagev1.ImageTag{},
		&osimagev1.ImageTagList{},
		&ImageSecretList{},
		&metav1.CreateOptions{},
		&metav1.ListOptions{},
		&metav1.UpdateOptions{},
		&metav1.DeleteOptions{},
	)
}
