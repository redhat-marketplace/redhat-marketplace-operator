package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MarketplaceConfigSpec defines the desired state of MarketplaceConfig
type MarketplaceConfigSpec struct {
  // RhmAccountID is the Red Hat Marketplace Account identifier
	RhmAccountID     string  `json:"rhmAccountID"`
  // ClusterUUID is the Red Hat Marketplace cluster identifier
	ClusterUUID      string  `json:"clusterUUID"`
  // DeploySecretName is the secret name that contains the deployment information
	DeploySecretName *string `json:"deploySecretName,omitempty"`
}

// MarketplaceConfigStatus defines the observed state of MarketplaceConfig
type MarketplaceConfigStatus struct {
  // Conditions represent the latest available observations of an object's stateonfig
	Conditions status.Conditions `json:"conditions"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MarketplaceConfig is configuration manager for our Red Hat Marketplace controllers
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=marketplaceconfigs,scope=Namespaced
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

  // Reasons for install
  ReasonStartInstall status.ConditionReason = "StartInstall"
  ReasonRazeeInstalled status.ConditionReason = "RazeeInstalled"
  ReasonMeterBaseInstalled status.ConditionReason = "MeterBaseInstalled"
  ReasonOperatorSourceInstall status.ConditionReason = "OperatorSourceInstalled"
  ReasonInstallFinished status.ConditionReason = "FinishedInstall"
)

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
