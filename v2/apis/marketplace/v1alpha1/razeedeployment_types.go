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
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Configuration values used by Razee to communicate with the Red Hat Marketplace backend
type RazeeConfigurationValues struct {
	// Api key used to access the bucket IBM COS
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	IbmCosReaderKey *corev1.SecretKeySelector `json:"ibmCosReaderKey,omitempty"`
	// BucketName is the name of the bucket in Cloud Object Storage and correlates to your accountID
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	BucketName string `json:"bucketName,omitempty"`
	// Base url for the instance of IBM COS
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	IbmCosURL string `json:"ibmCosUrl,omitempty"`
	// Key used to identify a particular razee instance
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	RazeeDashOrgKey *corev1.SecretKeySelector `json:"razeeDashOrgKey,omitempty"`
	// The file name of the child RemoteResourecS3
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	ChildRSS3FIleName string `json:"childRRS3FileName,omitempty"`
	// Url used by the razee install to post data
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	RazeeDashUrl string `json:"razeeDashUrl,omitempty"`
	// FileSourceURL DEPRECATED: The url of the filesource arg that gets passed into the razeedeploy-job
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	FileSourceURL *string `json:"fileSourceUrl,omitempty"`
}

// RazeeDeploymentSpec defines the desired state of RazeeDeployment
// +k8s:openapi-gen=true
type RazeeDeploymentSpec struct {

	// Enabled is the flag that controls if the controller does work. Setting
	// enabled to true will create a Razee namespace and deploy it's components. Set to false to bypass Razee installation
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Enabled bool `json:"enabled"`

	// ClusterUUID is the cluster identifier, used for installing razee.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	ClusterUUID string `json:"clusterUUID"`

	// DeploySecretName is the name of our secret where Razee
	// variables are stored.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	DeploySecretName *string `json:"deploySecretName,omitempty"`

	// TargetNamespace is configurable target of the razee namespace
	// This is to support legancy installs. Please do not edit.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	TargetNamespace *string `json:"targetNamespace,omitempty"`

	// Configuration values provided from redhat marketplace
	// These are used internally by the Operator
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	DeployConfig *RazeeConfigurationValues `json:"deployConfig,omitempty"`

	// Location of your IBM Cloud Object Storage resources
	// Used internally by the Operator
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	ChildUrl *string `json:"childUrl,omitempty"`
	// Flag used by the RazeeDeployment Controller to decide whether to run legacy uninstall job
	// Used internally by the Operator
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	LegacyUninstallHasRun *bool `json:"legacyUninstallHasRun,omitempty"`

	// InstallIBMCatalogSource is the flag that indicates if the IBM Catalog Source is installed
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Install IBM Catalog Source?"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	InstallIBMCatalogSource *bool `json:"installIBMCatalogSource,omitempty"`

	// The features that can be enabled or disabled
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Features"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	Features *common.Features `json:"features,omitempty"`

	// The ClusterDisplayName is a unique name of for a cluster specified by the admin during registration
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Features"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	ClusterDisplayName string `json:"clusterDisplayName,omitempty"`
}

// TODO: on version change, rename conditions to jobConditions
// TODO: on version change, rename installConditions to conditions

// RazeeDeploymentStatus defines the observed state of RazeeDeployment
// +k8s:openapi-gen=true
type RazeeDeploymentStatus struct {
	// RazeeConditions represent the latest available observations of an object's stateonfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Conditions status.Conditions `json:"installConditions,omitempty"`
	// JobConditions DEPRECATED: represent the latest available observations of an object's stateonfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	JobConditions *batch.JobCondition `json:"conditions,omitempty"`
	// JobState DEPRECATED: is the status of the Razee Install Job
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	JobState *batch.JobStatus `json:"jobState,omitempty"`
	// MissingValuesFromSecret validates the secret provided has all the correct fields
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	MissingDeploySecretValues []string `json:"missingValuesFromSecret,omitempty"`
	// RazeePrerequestesCreated is the list of configmaps and secrets required to be installed
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	RazeePrerequisitesCreated []string `json:"razeePrerequisitesCreated,omitempty"`
	// LocalSecretVarsPopulated DEPRECATED: informs if the correct local variables are correct set.
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	LocalSecretVarsPopulated *bool `json:"localSecretVarsPopulated,omitempty"`
	// RedHatMarketplaceSecretFound DEPRECATED: is the status of finding the secret in the cluster
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	RedHatMarketplaceSecretFound *bool `json:"redHatMarketplaceSecretFound,omitempty"`
	// RazeeJobInstall contains information regarding the install job so it can be removed
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	RazeeJobInstall *RazeeJobInstallStruct `json:"razee_job_install,omitempty"`
	// NodesFromRazeeDeployments contains the pods names created by the rhm-watch-keeper and rhm-remote-resources3-controller deployments
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	NodesFromRazeeDeployments []string `json:"nodesFromRazeeDeployments,omitempty"`
	// NodesFromRazeeDeploymentsCount contains the pods names created by the rhm-watch-keeper and rhm-remote-resources3-controller deployments
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	NodesFromRazeeDeploymentsCount int `json:"nodesFromRazeeDeploymentsCount,omitempty"`
}

type RazeeJobInstallStruct struct {
	// RazeeNamespace is the namespace targeted for the Razee install
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	RazeeNamespace string `json:"razee_namespace"`
	// RazeeInstallURL is the url used to install the Razee resources
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	RazeeInstallURL string `json:"razee_install_url"`
}

// +kubebuilder:object:root=true
// RazeeDeployment is the resources that deploys Razee for the Red Hat Marketplace.
// This is an internal resource not meant to be modified directly.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=razeedeployments,scope=Namespaced
// +kubebuilder:printcolumn:name="INSTALLING",type=string,JSONPath=`.status.installConditions[?(@.type == "Installing")].status`
// +kubebuilder:printcolumn:name="STEP",type=string,JSONPath=`.status.installConditions[?(@.type == "Installing")].reason`
// +kubebuilder:printcolumn:name="APPS",type=integer,JSONPath=`.status.nodesFromRazeeDeploymentsCount`
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="(Internal) Razee Deployment"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Deployment,v1,"redhat-marketplace-operator"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ConfigMap,v1,"redhat-marketplace-operator"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Secret,v1,"redhat-marketplace-operator"`
type RazeeDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RazeeDeploymentSpec   `json:"spec,omitempty"`
	Status RazeeDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RazeeDeploymentList contains a list of RazeeDeployment
type RazeeDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RazeeDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RazeeDeployment{}, &RazeeDeploymentList{})
}

// These are valid conditions of RazeeDeployment
const (

	// Reasons for install
	ReasonRazeeStartInstall                      status.ConditionReason = "StartRazeeInstall"
	ReasonWatchKeeperNonNamespacedInstalled      status.ConditionReason = "FinishedWatchKeeperNonNamespaceInstall"
	ReasonWatchKeeperLimitPollInstalled          status.ConditionReason = "FinishedWatchKeeperLimitPollInstall"
	ReasonRazeeClusterMetaDataInstalled          status.ConditionReason = "FinishedRazeeClusterMetaDataInstall"
	ReasonWatchKeeperConfigInstalled             status.ConditionReason = "FinishedWatchKeeperConfigInstall"
	ReasonWatchKeeperSecretInstalled             status.ConditionReason = "FinishedWatchKeeperSecretInstall"
	ReasonCosReaderKeyInstalled                  status.ConditionReason = "FinishedCosReaderKeyInstall"
	ReasonRazeeDeployJobStart                    status.ConditionReason = "StartRazeeDeployJob"
	ReasonRazeeDeployJobFinished                 status.ConditionReason = "FinishedRazeeDeployJob"
	ReasonParentRRS3Installed                    status.ConditionReason = "FinishParentRRS3Install"
	ReasonChildRRS3Migrated                      status.ConditionReason = "ChildRRS3Migrated"
	ReasonRazeeInstallFinished                   status.ConditionReason = "FinishedRazeeInstall"
	ReasonWatchKeeperDeploymentStart             status.ConditionReason = "StartWatchKeeperDeploymentInstall"
	ReasonWatchKeeperDeploymentInstalled         status.ConditionReason = "FinishedWatchKeeperDeploymentInstall"
	ReasonRhmRemoteResourceS3DeploymentStart     status.ConditionReason = "StartRemoteResourceS3DeploymentInstall"
	ReasonRhmRemoteResourceS3DeploymentInstalled status.ConditionReason = "FinishedRemoteResourceS3DeploymentInstall"
	ReasonRhmRemoteResourceS3DeploymentEnabled   status.ConditionReason = "EnabledRemoteResourceS3DeploymentInstall"
	ReasonRhmRegistrationWatchkeeperEnabled      status.ConditionReason = "EnabledRegistrationWatchkeeperInstall"
)

var (
	ConditionWatchKeeperNonNamespacedInstalled = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonWatchKeeperNonNamespacedInstalled,
		Message: "watch-keeper-non-namespaced install finished",
	}

	ConditionWatchKeeperLimitPollInstalled = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonWatchKeeperLimitPollInstalled,
		Message: "watch-keeper-limit-poll install finished",
	}

	ConditionRazeeClusterMetaDataInstalled = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonRazeeClusterMetaDataInstalled,
		Message: "Razee cluster meta data install finished",
	}

	ConditionWatchKeeperConfigInstalled = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonWatchKeeperConfigInstalled,
		Message: "watch-keeper-config install finished",
	}

	ConditionWatchKeeperSecretInstalled = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonWatchKeeperSecretInstalled,
		Message: "watch-keeper-secret install finished",
	}

	ConditionCosReaderKeyInstalled = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonCosReaderKeyInstalled,
		Message: "Cos-reader-key install finished",
	}

	ConditionRazeeInstallFinished = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionFalse,
		Reason:  ReasonRazeeInstallFinished,
		Message: "Razee install complete",
	}

	ConditionRazeeInstallComplete = status.Condition{
		Type:    ConditionComplete,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonRazeeInstallFinished,
		Message: "Razee install complete",
	}

	ConditionRazeeNotEnabled = status.Condition{
		Type:    ConditionComplete,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonRazeeInstallFinished,
		Message: "Razee not enabled",
	}

	ConditionRazeeNameMismatch = status.Condition{
		Type:    ConditionComplete,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonRazeeInstallFinished,
		Message: "RazeeDeploy Resource name does not match expected",
	}

	ConditionRazeeStartInstall = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonRazeeStartInstall,
		Message: "Razee Install starting",
	}

	ConditionResourceS3DeploymentDisabled = status.Condition{
		Type:    ConditionDeploymentEnabled,
		Status:  corev1.ConditionFalse,
		Reason:  ReasonRhmRemoteResourceS3DeploymentEnabled,
		Message: "Deployment feature is disabled. RemoteResourceS3 deployment disabled. Operator deployment will be unavailable on marketplace.redhat.com",
	}

	ConditionRhmRegistrationWatchkeeperDisabled = status.Condition{
		Type:    ConditionRegistrationEnabled,
		Status:  corev1.ConditionFalse,
		Reason:  ReasonRhmRegistrationWatchkeeperEnabled,
		Message: "Registration feature is disabled. WatchKeeper disabled. Registration status will be unavailable on marketplace.redhat.com",
	}

	ConditionRhmRemoteResourceS3DeploymentEnabled = status.Condition{
		Type:    ConditionDeploymentEnabled,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonRhmRemoteResourceS3DeploymentEnabled,
		Message: "RemoteResourceS3 deployment enabled",
	}

	ConditionRhmRegistrationWatchkeeperEnabled = status.Condition{
		Type:    ConditionRegistrationEnabled,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonRhmRegistrationWatchkeeperEnabled,
		Message: "Registration deployment enabled",
	}

	ConditionParentRRS3Installed = status.Condition{
		Type:    ConditionInstalling,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonParentRRS3Installed,
		Message: "ParentRRS3 install finished",
	}

	ConditionChildRRS3MigrationComplete = status.Condition{
		Type:    ConditionComplete,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonChildRRS3Migrated,
		Message: "Child RRS3 Migration successful",
	}
)
