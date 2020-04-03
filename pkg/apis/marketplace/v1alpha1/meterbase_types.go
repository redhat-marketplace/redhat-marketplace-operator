package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StorageSpec contains configuration for pvc claims.
type StorageSpec struct {
	// Storage class for the prometheus stateful set. Default is "" i.e. default.
	// +optional
	Class *string `json:"class,omitempty"`
	// Storage size for the prometheus deployment. Default is 40Gi.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=quantity
	Size resource.Quantity `json:"size,omitempty"`
}

// PrometheusSpec contains configuration regarding prometheus
// deployment used for metering.
type PrometheusSpec struct {
	// Resource requirements for the deployment. Default is not defined.
	// +optional
	corev1.ResourceRequirements `json:"resources,omitempty"`

	// Selector for the pods in the Prometheus deployment
	// +optional
	NodeSelector map[string]string `json:"selector,omitempty"`

	// Storage for the deployment.
	Storage StorageSpec `json:"storage"`
}

// MeterBaseSpec defines the desired state of MeterBase
type MeterBaseSpec struct {
	// Is metering is enabled on the cluster? Default is true
	Enabled bool `json:"enabled"`

	// Prometheus deployment configuration.
	// +optional
	Prometheus *PrometheusSpec `json:"prometheus,omitempty"`
}

// MeterBaseStatus defines the observed state of MeterBase.
type MeterBaseStatus struct {
	// PrometheusNodes are the names of the prometheus pods
	// +listType=set
	PrometheusNodes []string          `json:"prometheusNodes,omitempty"`
	Conditions      status.Conditions `json:"conditions"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MeterBase is the Schema for the meterbases API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=meterbases,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="(Internal) Meter Configuration"
type MeterBase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterBaseSpec   `json:"spec,omitempty"`
	Status MeterBaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MeterBaseList contains a list of MeterBase
type MeterBaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterBase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeterBase{}, &MeterBaseList{})
}
