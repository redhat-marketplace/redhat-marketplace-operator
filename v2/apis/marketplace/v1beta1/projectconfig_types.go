package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntimecfg "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	// "github.com/mitchellh/mapstructure"
)

// +kubebuilder:object:root=true
// ProjectConfig is the Schema for the projectconfigs API
type ProjectConfig struct {
	metav1.TypeMeta `json:",inline" mapstructure:",inline"`

	// ControllerManagerConfigurationSpec returns the configurations for controllers
	controllerruntimecfg.ControllerManagerConfigurationSpec `json:",inline" mapstructure:",squash"`

	ClusterName string `json:"clusterName,omitempty" mapstructure:"clusterName,omitempty"`

	ComponentConfigMarketplaceConfigSpec ComponentConfigMarketplaceConfigSpec `json:"marketplaceConfigSpec,omitempty" mapstructure:"marketplaceConfigSpec,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ProjectConfig{})
}
