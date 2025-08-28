// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageSpec contains configuration for pvc claims.
type StorageSpec struct {
	// Storage class for the prometheus stateful set. Default is "" i.e. default.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Class *string `json:"class,omitempty"`

	// Storage size for the prometheus deployment. Default is 40Gi.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=quantity
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Size resource.Quantity `json:"size,omitempty"`

	// EmptyDir is a temporary storage type that gets created on the prometheus pod. When this is defined metering will run on CRC.
	// +kubebuilder:validation:Type=object
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`
}

// PrometheusSpec contains configuration regarding prometheus
// deployment used for metering.
type PrometheusSpec struct {
	// Resource requirements for the deployment. Default is not defined.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	corev1.ResourceRequirements `json:"resources,omitempty"`

	// Selector for the pods in the Prometheus deployment
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	NodeSelector map[string]string `json:"selector,omitempty"`

	// Storage for the deployment.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Storage StorageSpec `json:"storage"`

	// Replicas defines the number of desired replicas for the prometheus deployment. Used primarily when running metering on CRC
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

type ExternalPrometheus struct {
}

// MeterBaseSpec defines the desired state of MeterBase
// +k8s:openapi-gen=true
type MeterBaseSpec struct {
	// Enabled is the flag that controls if the controller does work. Setting
	// enabled to "true" will install metering components. False will suspend controller
	// operations for metering components.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Enabled bool `json:"enabled"`

	// Prometheus deployment configuration.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Prometheus *PrometheusSpec `json:"prometheus,omitempty"`

	// [DEPRECATED] MeterdefinitionCatalogServerConfig holds configuration for the Meterdefinition Catalog Server.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	MeterdefinitionCatalogServerConfig *common.MeterDefinitionCatalogServerConfig `json:"meterdefinitionCatalogServerConfig,omitempty"`

	// AdditionalConfigs are set by meter definitions and meterbase to what is available on the
	// system.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	AdditionalScrapeConfigs *corev1.SecretKeySelector `json:"additionalScrapeConfigs,omitempty"`

	// DataServiceEnabled is the flag that controls if the DataService will be created.
	// Setting enabled to "true" will install DataService components.
	// False will delete the DataServicecomponents.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	DataServiceEnabled *bool `json:"dataServiceEnabled,omitempty"`

	// UserWorkloadMonitoringEnabled controls whether to attempt to use
	// Openshift user-defined workload monitoring as the Prometheus provider
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	UserWorkloadMonitoringEnabled *bool `json:"userWorkloadMonitoringEnabled,omitempty"`

	// storageClassName is the name of the StorageClass required by the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Storage Class Name"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="text"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty" protobuf:"bytes,5,opt,name=storageClassName"`
}

func (m *MeterBaseSpec) IsDataServiceEnabled() bool {
	if m.DataServiceEnabled == nil {
		return true
	}

	return *m.DataServiceEnabled
}

// MeterBaseStatus defines the observed state of MeterBase.
// +k8s:openapi-gen=true
type MeterBaseStatus struct {
	// MeterBaseConditions represent the latest available observations of an object's stateonfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`
	// PrometheusStatus is the most recent observed status of the Prometheus cluster. Read-only. Not
	// included when requesting from the apiserver, only from the Prometheus
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	PrometheusStatus *monitoringv1.PrometheusStatus `json:"prometheusStatus,omitempty"`

	// [DEPRECATED] MeterdefinitionCatalogServerStatus is the most recent observed status of the Meterdefinition Catalog Server. Read-only. Not
	// included when requesting from the apiserver, only from the Prometheus
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	MeterdefinitionCatalogServerStatus *MeterdefinitionCatalogServerStatus `json:"meterdefinitionCatalogServerStatus,omitempty"`

	// Total number of non-terminated pods targeted by this Prometheus deployment
	// (their labels match the selector).
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Total number of non-terminated pods targeted by this Prometheus deployment
	// that have the desired version spec.
	// +optional
	UpdatedReplicas *int32 `json:"updatedReplicas,omitempty"`
	// Total number of available pods (ready for at least minReadySeconds)
	// targeted by this Prometheus deployment.
	// +optional
	AvailableReplicas *int32 `json:"availableReplicas,omitempty"`
	// Total number of unavailable pods targeted by this Prometheus deployment.
	// +optional
	UnavailableReplicas *int32 `json:"unavailableReplicas,omitempty"`
	// Targets is a list of prometheus activeTargets
	// +optional
	Targets []common.Target `json:"targets,omitempty"`
}

// [DEPRECATED] MeterdefinitionCatalogServerStatus defines the observed state of the MeterdefinitionCatalogServer.
// +k8s:openapi-gen=true
type MeterdefinitionCatalogServerStatus struct {
	// MeterdefinitionCatalogServerConditions represent the latest available observations of an object's stateonfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`
}

// MeterBase is the resource that sets up Metering for Red Hat Marketplace.
// This is an internal resource not meant to be modified directly.
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="INSTALLING",type=string,JSONPath=`.status.conditions[?(@.type == "Installing")].status`
// +kubebuilder:printcolumn:name="STEP",type=string,JSONPath=`.status.conditions[?(@.type == "Installing")].reason`
// +kubebuilder:printcolumn:name="AvailableReplicas",type=integer,JSONPath=`.status.availableReplicas`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`
// +kubebuilder:printcolumn:name="UpdatedReplicas",type=integer,JSONPath=`.status.updatedReplicas`
// +kubebuilder:printcolumn:name="UnavailableReplicas",type=integer,JSONPath=`.status.unavailableReplicas`
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=meterbases,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Metering"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ServiceMonitor,v1,"redhat-marketplace-operator"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Prometheus,v1,"redhat-marketplace-operator"`
type MeterBase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterBaseSpec   `json:"spec,omitempty"`
	Status MeterBaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MeterBaseList contains a list of MeterBase
type MeterBaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterBase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeterBase{}, &MeterBaseList{})
}

const (
	PrometheusTargetsHealth status.ConditionType = "Health"

	// Reasons for install
	ReasonMeterBaseStartInstall             status.ConditionReason = "StartMeterBaseInstall"
	ReasonMeterBasePrometheusInstall        status.ConditionReason = "StartMeterBasePrometheusInstall"
	ReasonMeterBasePrometheusServiceInstall status.ConditionReason = "StartMeterBasePrometheusServiceInstall"
	ReasonMeterBaseFinishInstall            status.ConditionReason = "FinishedMeterBaseInstall"

	// User Workload Monitoring
	// ConditionUserWorkloadMonitoringEnabled means UWM is actively used as the prometheus provider
	ConditionUserWorkloadMonitoringEnabled status.ConditionType = "UserWorkloadMonitoringEnabled"

	ReasonUserWorkloadMonitoringEnabled                        status.ConditionReason = "UserWorkloadMonitoringEnabled"
	ReasonUserWorkloadMonitoringSpecDisabled                   status.ConditionReason = "UserWorkloadMonitoringSpecDisabled"
	ReasonUserWorkloadMonitoringClusterDisabled                status.ConditionReason = "UserWorkloadMonitoringClusterDisabled"
	ReasonUserWorkloadMonitoringInsufficientStorage            status.ConditionReason = "UserWorkloadMonitoringInsufficientStorage"
	ReasonUserWorkloadMonitoringRetentionTime                  status.ConditionReason = "UserWorkloadMonitoringRetentionTime"
	ReasonUserWorkloadMonitoringParseUserWorkloadConfiguration status.ConditionReason = "UserWorkloadMonitoringParseUserWorkloadConfiguration"
	ReasonUserWorkloadMonitoringConfigNotFound                 status.ConditionReason = "UserWorkloadMonitoringConfigNotFound"
	ReasonUserWorkloadMonitoringTransitioning                  status.ConditionReason = "UserWorkloadMonitoringTransitioning"

	MessageUserWorkloadMonitoringEnabled         string = "UserWorkloadMonitoring is enabled in the Meterbase Spec, is enabled on the Cluster, and user-workload-monitoring configmap is configured correctly"
	MessageUserWorkloadMonitoringSpecDisabled    string = "UserWorkloadMonitoring is disabled in the Meterbase Spec"
	MessageUserWorkloadMonitoringClusterDisabled string = "UserWorkloadMonitoring is unavailable or disabled on the Cluster"
	MessageUserWorkloadMonitoringTransitioning   string = "Transitioning between UserWorkloadMonitoring and RHM prometheus provider"

	// Reasons for health
	ReasonMeterBasePrometheusTargetsHealthBad  status.ConditionReason = "HealthBad Targets in Status"
	ReasonMeterBasePrometheusTargetsHealthGood status.ConditionReason = "HealthGood"
)

var (
	MeterBasePrometheusTargetBadHealth = status.Condition{
		Type:    PrometheusTargetsHealth,
		Status:  corev1.ConditionFalse,
		Reason:  ReasonMeterBasePrometheusTargetsHealthBad,
		Message: "Prometheus activeTargets contains targets with HealthBad or HealthUnknown.",
	}
	MeterBasePrometheusTargetGoodHealth = status.Condition{
		Type:    PrometheusTargetsHealth,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonMeterBasePrometheusTargetsHealthGood,
		Message: "Prometheus activeTargets contains targets with HealthGood.",
	}

	UserWorkloadMonitoringEnabled = status.Condition{
		Type:    ConditionUserWorkloadMonitoringEnabled,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonUserWorkloadMonitoringEnabled,
		Message: MessageUserWorkloadMonitoringEnabled,
	}

	UserWorkloadMonitoringDisabledSpec = status.Condition{
		Type:    ConditionUserWorkloadMonitoringEnabled,
		Status:  corev1.ConditionFalse,
		Reason:  ReasonUserWorkloadMonitoringSpecDisabled,
		Message: MessageUserWorkloadMonitoringSpecDisabled,
	}

	UserWorkloadMonitoringDisabledOnCluster = status.Condition{
		Type:    ConditionUserWorkloadMonitoringEnabled,
		Status:  corev1.ConditionFalse,
		Reason:  ReasonUserWorkloadMonitoringClusterDisabled,
		Message: MessageUserWorkloadMonitoringClusterDisabled,
	}

	UserWorkloadMonitoringStorageConfigurationErr = status.Condition{
		Type:   ConditionUserWorkloadMonitoringEnabled,
		Status: corev1.ConditionFalse,
		Reason: ReasonUserWorkloadMonitoringInsufficientStorage,
		// Message: userWorkloadConfigurationErr.Error(),
	}

	UserWorkloadMonitoringRetentionTimeConfigurationErr = status.Condition{
		Type:   ConditionUserWorkloadMonitoringEnabled,
		Status: corev1.ConditionFalse,
		Reason: ReasonUserWorkloadMonitoringRetentionTime,
		// Message: userWorkloadConfigurationErr.Error(),
	}

	UserWorkloadMonitoringParseUserWorkloadConfigurationErr = status.Condition{
		Type:   ConditionUserWorkloadMonitoringEnabled,
		Status: corev1.ConditionFalse,
		Reason: ReasonUserWorkloadMonitoringParseUserWorkloadConfiguration,
		// Message: userWorkloadConfigurationErr.Error(),
	}

	UserWorkloadMonitoringConfigNotFound = status.Condition{
		Type:   ConditionUserWorkloadMonitoringEnabled,
		Status: corev1.ConditionFalse,
		Reason: ReasonUserWorkloadMonitoringConfigNotFound,
		// Message: userWorkloadConfigurationErr.Error(),
	}

	MeterBaseStartInstall = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonMeterBaseStartInstall,
		Message: "MeterBase install started",
	}

	MeterBaseFinishInstall = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionFalse,
		Reason:  ReasonMeterBaseFinishInstall,
		Message: "MeterBase install complete",
	}
)
