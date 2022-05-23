package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntimecfg "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
)

// +kubebuilder:object:root=true
// ProjectConfig is the Schema for the projectconfigs API
type ProjectConfig struct {
	metav1.TypeMeta `json:",inline"`

	// ControllerManagerConfigurationSpec returns the contfigurations for controllers
	controllerruntimecfg.ControllerManagerConfigurationSpec `json:",inline"`

	ClusterName string `json:"clusterName,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ProjectConfig{})
}
