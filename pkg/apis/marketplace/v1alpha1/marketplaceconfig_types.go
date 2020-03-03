package v1alpha1

import (
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/shared"
	"github.com/operator-framework/operator-sdk/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MarketplaceConfigSpec defines the desired state of MarketplaceConfig
type MarketplaceConfigSpec struct {
	Size int32 `json:"size"`

	// The name of the OperatorSource that the packages originate from
	Source          string `json:"source"`
	TargetNamespace string `json:"targetNamespace"`
	Packages        string `json:"packages"`

	// DisplayName is passed along to the CatalogSource to be used
	// as a pretty name.
	DisplayName string `json:"csDisplayName,omitempty"`

	// Publisher is passed along to the CatalogSource to be used
	// to define what entity published the artifacts from the OperatorSource.
	Publisher string `json:"csPublisher,omitempty"`
}

// MarketplaceConfigStatus defines the observed state of MarketplaceConfig
type MarketplaceConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Conditions status.Conditions `json:"conditions"`

	// Current phase of the CatalogSourceConfig object.
	CurrentPhase shared.ObjectPhase `json:"currentPhase,omitempty"`

	// Map of packages (key) and their app registry package version (value)
	PackageRepositioryVersions map[string]string `json:"packageRepositioryVersions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MarketplaceConfig is the Schema for the marketplaceconfigs API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=marketplaceconfigs,scope=Namespaced
type MarketplaceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MarketplaceConfigSpec   `json:"spec,omitempty"`
	Status MarketplaceConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MarketplaceConfigList contains a list of MarketplaceConfig
type MarketplaceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MarketplaceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MarketplaceConfig{}, &MarketplaceConfigList{})
}
