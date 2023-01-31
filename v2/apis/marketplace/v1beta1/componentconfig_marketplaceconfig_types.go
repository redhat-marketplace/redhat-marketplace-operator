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

package v1beta1

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentConfigMarketplaceConfigSpec defines the desired state of MarketplaceConfig
// +k8s:openapi-gen=true
type ComponentConfigMarketplaceConfigSpec struct {
	// RhmAccountID is the Red Hat Marketplace Account identifier
	RhmAccountID string `json:"rhmAccountID,omitempty"`

	// ClusterUUID is the Red Hat Marketplace cluster identifier
	ClusterUUID string `json:"clusterUUID,omitempty"`

	// ClusterName is the name that will be assigned to your cluster in the Red Hat Marketplace UI.
	// If you have set the name in the UI first, this name will be ignored.
	ClusterName string `json:"clusterName,omitempty"`

	// DeploySecretName is the secret name that contains the deployment information
	DeploySecretName *string `json:"deploySecretName,omitempty"`

	// EnableMetering enables the Marketplace Metering components
	EnableMetering *bool `json:"enableMetering,omitempty"`

	// InstallIBMCatalogSource is the flag that indicates if the IBM Catalog Source is installed
	InstallIBMCatalogSource *bool `json:"installIBMCatalogSource,omitempty"`

	// The features that can be enabled or disabled
	Features *common.Features `json:"features,omitempty"`

	// NamespaceLabelSelector is a LabelSelector that overrides the default LabelSelector used to build the OperatorGroup Namespace list
	NamespaceLabelSelector *metav1.LabelSelector `json:"namespaceLabelSelector,omitempty"`
}

// ComponentConfigMarketplaceConfigStatus defines the observed state of MarketplaceConfig
// +k8s:openapi-gen=true
type ComponentConfigMarketplaceConfigStatus struct {
	// Conditions represent the latest available observations of an object's stateonfig
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`

	// RazeeSubConditions represent the latest available observations of the razee object's state
	// +optional
	RazeeSubConditions status.Conditions `json:"razeeSubConditions,omitempty"`

	// MeterBaseSubConditions represent the latest available observations of the meterbase object's state
	// +optional
	MeterBaseSubConditions status.Conditions `json:"meterBaseSubConditions,omitempty"`
}

// ComponentConfigMarketplaceConfig is configuration manager for our Red Hat Marketplace controllers
// By installing this product you accept the license terms https://ibm.biz/BdfaAY.
// +kubebuilder:object:root=true
type ComponentConfigMarketplaceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentConfigMarketplaceConfigSpec   `json:"spec,omitempty"`
	Status ComponentConfigMarketplaceConfigStatus `json:"status,omitempty"`
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

	// ConditionIsDisconnected means the rhm operator is running in a disconnected environment
	ConditionIsDisconnected status.ConditionType = "IsDisconnected"

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

func init() {
	SchemeBuilder.Register(&ComponentConfigMarketplaceConfig{})
}
