package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
)

// MeterDefinitionSpec defines the desired metering spec
type MeterDefinitionSpec struct {
	// ServiceLabels of the meterics you want to track.
	ServiceMeterLabels []string `json:"serviceMeterLabels"`

	// PodLabels of the prometheus metrics you want to track.
	PodMeterLabels []string `json:"podMeterLabels"`

	// ServiceMonitors to be selected for target discovery.
	ServiceMonitorSelector *metav1.LabelSelector `json:"serviceMonitorSelector,omitempty"`

	// Namespaces to be selected for ServiceMonitor discovery. If nil, only
	// check own namespace.
	ServiceMonitorNamespaceSelector *metav1.LabelSelector `json:"serviceMonitorNamespaceSelector,omitempty"`

	// *Experimental* PodMonitors to be selected for target discovery.
	PodSelector *metav1.LabelSelector `json:"podMonitorSelector,omitempty"`

	// Namespaces to be selected for Pod discovery. If nil, only
	// check own namespace.
	PodNamespaceSelector *metav1.LabelSelector `json:"podMonitorNamespaceSelector,omitempty"`
}

// MeterDefinitionStatus defines the observed state of MeterDefinition
type MeterDefinitionStatus struct {

	// ServiceLabels of the meterics you want to track.
	ServiceLabels []string `json:"serviceLabels"`

	// PodLabels of the prometheus metrics you want to track.
	PodLabels []string `json:"podLabels"`

	// ServiceMonitors is the list of service monitors being watched for
	// this meter definition
	ServiceMonitors []*monitoringv1.ServiceMonitor `json:"serviceMonitors"`

	// PodMonitors is the list of current pod mointors being watched for
	// this meter definition
	Pods []*metav1.ObjectMeta `json:"podMonitors"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MeterDefinition is the Schema for the meterdefinitions API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=meterdefinitions,scope=Namespaced
type MeterDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterDefinitionSpec   `json:"spec,omitempty"`
	Status MeterDefinitionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MeterDefinitionList contains a list of MeterDefinition
type MeterDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeterDefinition{}, &MeterDefinitionList{})
}
