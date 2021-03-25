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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MarketplaceConfigSpec defines the desired state of MarketplaceConfig
// +k8s:openapi-gen=true
type MarketplaceConfigSpec struct {
	// RhmAccountID is the Red Hat Marketplace Account identifier
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Marketplace Accound ID"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="text"
	RhmAccountID string `json:"rhmAccountID"`

	// ClusterUUID is the Red Hat Marketplace cluster identifier
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	ClusterUUID string `json:"clusterUUID"`

	// ClusterName is the name that will be assigned to your cluster in the Red Hat Marketplace UI.
	// If you have set the name in the UI first, this name will be ignored.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Marketplace Cluster Name"
	ClusterName string `json:"clusterName,omitempty"`

	// DeploySecretName is the secret name that contains the deployment information
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Marketplace Secret Name"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="Secret"
	DeploySecretName *string `json:"deploySecretName,omitempty"`

	// EnableMetering enables the Marketplace Metering components
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	EnableMetering *bool `json:"enableMetering,omitempty"`

	// InstallIBMCatalogSource is the flag that indicates if the IBM Catalog Source is installed
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Install IBM Catalog Source?"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	InstallIBMCatalogSource *bool `json:"installIBMCatalogSource,omitempty"`

	// The features that can be enabled or disabled
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Disabled Features"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	Features *common.Features `json:"features,omitempty"`

	// NamespaceLabelSelector is a LabelSelector that overrides the default LabelSelector used to build the OperatorGroup Namespace list
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Namespace LabelSelector"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	NamespaceLabelSelector *metav1.LabelSelector `json:"namespaceLabelSelector,omitempty"`
}

// MarketplaceConfigStatus defines the observed state of MarketplaceConfig
// +k8s:openapi-gen=true
type MarketplaceConfigStatus struct {
	// Conditions represent the latest available observations of an object's stateonfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`

	// RazeeSubConditions represent the latest available observations of the razee object's state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	RazeeSubConditions status.Conditions `json:"razeeSubConditions,omitempty"`

	// MeterBaseSubConditions represent the latest available observations of the meterbase object's state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	MeterBaseSubConditions status.Conditions `json:"meterBaseSubConditions,omitempty"`
}

// MarketplaceConfig is configuration manager for our Red Hat Marketplace controllers
// +kubebuilder:object:root=true
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=marketplaceconfigs,scope=Namespaced
// +kubebuilder:printcolumn:name="INSTALLING",type=string,JSONPath=`.status.conditions[?(@.type == "Installing")].status`
// +kubebuilder:printcolumn:name="STEP",type=string,JSONPath=`.status.conditions[?(@.type == "Installing")].reason`
// +kubebuilder:printcolumn:name="REGISTERED",type=string,JSONPath=`.status.conditions[?(@.type == "Registered")].status`
// +kubebuilder:printcolumn:name="REGISTERED_MSG",type=string,JSONPath=`.status.conditions[?(@.type == "Registered")].message`
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Marketplace"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`RazeeDeployment,v1alpha1,"redhat-marketplace-operator"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`OperatorSource,v1,"redhat-marketplace-operator"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`MeterBase,v1alpha1,"redhat-marketplace-operator"`
type MarketplaceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MarketplaceConfigSpec   `json:"spec,omitempty"`
	Status MarketplaceConfigStatus `json:"status,omitempty"`
}

// These are valid conditions of a job.
const (
	// ConditionInstalling means the tools are installing on your cluster.
	ConditionInstalling status.ConditionType = "Installing"
	// ConditionComplete means the tools are installed on your cluster.
	ConditionComplete status.ConditionType = "Complete"
	// ConditionError means the installation has failed.
	ConditionError status.ConditionType = "Error"
	// ConditionRegistered means the cluster registered.
	ConditionRegistered status.ConditionType = "Registered"
	// ConditionRegistered means the cluster registered.
	ConditionRegistrationError status.ConditionType = "RegistationError"

	// Reasons for install
	ReasonStartInstall          status.ConditionReason = "StartInstall"
	ReasonRazeeInstalled        status.ConditionReason = "RazeeInstalled"
	ReasonMeterBaseInstalled    status.ConditionReason = "MeterBaseInstalled"
	ReasonOperatorSourceInstall status.ConditionReason = "OperatorSourceInstalled"
	ReasonCatalogSourceInstall  status.ConditionReason = "CatalogSourceInstalled"
	ReasonCatalogSourceDelete   status.ConditionReason = "CatalogSourceDeleted"
	ReasonInstallFinished       status.ConditionReason = "FinishedInstall"
	ReasonRegistrationSuccess   status.ConditionReason = "ClusterRegistered"
	ReasonRegistrationFailure   status.ConditionReason = "ClusterNotRegistered"
	ReasonServiceUnavailable    status.ConditionReason = "ServiceUnavailable"
	ReasonInternetDisconnected  status.ConditionReason = "InterntNotAvailable"
	ReasonClientError           status.ConditionReason = "ClientError"
	ReasonRegistrationError     status.ConditionReason = "HttpError"
	ReasonOperatingNormally     status.ConditionReason = "OperatingNormally"
	ReasonNoError               status.ConditionReason = ReasonOperatingNormally

	// Enablement/Disablement of features conditions
	// ConditionDeploymentEnabled means the particular option is enabled
	ConditionDeploymentEnabled status.ConditionType = "DeploymentEnabled"
	// ConditionRegistrationEnabled means the particular option is enabled
	ConditionRegistrationEnabled status.ConditionType = "RegistrationEnabled"
)

// +kubebuilder:object:root=true

// MarketplaceConfigList contains a list of MarketplaceConfig
type MarketplaceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MarketplaceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MarketplaceConfig{}, &MarketplaceConfigList{})
}
