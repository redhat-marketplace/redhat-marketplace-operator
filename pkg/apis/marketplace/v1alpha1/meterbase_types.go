package v1alpha1

import (
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
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
// +k8s:openapi-gen=true
type MeterBaseSpec struct {
	// Is metering is enabled on the cluster? Default is true
	Enabled bool `json:"enabled"`

	// Prometheus deployment configuration.
	// +optional
	Prometheus *PrometheusSpec `json:"prometheus,omitempty"`
}

// MeterBaseStatus defines the observed state of MeterBase.
// +k8s:openapi-gen=true
type MeterBaseStatus struct {
	// PrometheusStatus is the most recent observed status of the Prometheus cluster. Read-only. Not
	// included when requesting from the apiserver, only from the Prometheus
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	PrometheusStatus *monitoringv1.PrometheusStatus `json:"prometheusStatus,omitempty"`
}


// MeterBase is the resource that sets up Metering for Red Hat Marketplace.
// This is an internal resource not meant to be modified directly.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=meterbases,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="(Internal) Meter Config"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ServiceMonitor,v1,"redhat-marketplace-operator"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Prometheus,v1,"redhat-marketplace-operator"`
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
