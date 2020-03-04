package v1alpha1

import (
	// "github.com/operator-framework/operator-sdk/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batch "k8s.io/api/batch/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RazeeDeploymentSpec defines the desired state of RazeeDeployment
type RazeeDeploymentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// Setting enabled to "true" will create a Razee namespace and deploy it's componenets. Set to "false" to bypass Razee installation
	Enabled bool `json:"enabled"`
}

// RazeeDeploymentStatus defines the observed state of RazeeDeployment
type RazeeDeploymentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Conditions batch.JobCondition `json:"conditions"`
	JobState   batch.JobStatus    `json:"jobState"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RazeeDeployment is the Schema for the razeedeployments API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=razeedeployments,scope=Namespaced
type RazeeDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RazeeDeploymentSpec   `json:"spec,omitempty"`
	Status RazeeDeploymentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RazeeDeploymentList contains a list of RazeeDeployment
type RazeeDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RazeeDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RazeeDeployment{}, &RazeeDeploymentList{})
}
