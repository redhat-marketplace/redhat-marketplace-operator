package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestPersistenVolumeClaim(t *testing.T) {
	pvc, err := NewPersistentVolumeClaim(PersistentVolume{})

	if err != nil {
		t.Errorf("failed with error %v", err)
	}

	if len(pvc.Spec.AccessModes) == 0 {
		t.Error("no defined access modes")
	}

	if pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("expect %v but got %v", corev1.ReadWriteOnce, pvc.Spec.AccessModes[0])
	}

	val := corev1.ReadWriteMany
	pvc, err = NewPersistentVolumeClaim(PersistentVolume{
		AccessMode: &val,
	})

	if err != nil {
		t.Errorf("failed with error %v", err)
	}

	if len(pvc.Spec.AccessModes) == 0 {
		t.Error("no defined access modes")
	}

	if pvc.Spec.AccessModes[0] != corev1.ReadWriteMany {
		t.Errorf("expect %v but got %v", corev1.ReadWriteMany, pvc.Spec.AccessModes[0])
	}
}
