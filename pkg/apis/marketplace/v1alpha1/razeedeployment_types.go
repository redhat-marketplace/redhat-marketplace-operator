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
	status "github.com/operator-framework/operator-sdk/pkg/status"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Configuration values used by Razee to communicate with the Red Hat Marketplace backend
type RazeeConfigurationValues struct {
	// Api key used to access the bucket IBM COS
	IbmCosReaderKey *corev1.SecretKeySelector `json:"ibmCosReaderKey,omitempty"`
	// BucketName is the name of the bucket in Cloud Object Storage and correlates to your accountID
	BucketName string `json:"bucketName,omitempty"`
	// Base url for the instance of IBM COS
	IbmCosURL string `json:"ibmCosUrl,omitempty"`
	// Key used to identify a particular razee instance
	RazeeDashOrgKey *corev1.SecretKeySelector `json:"razeeDashOrgKey,omitempty"`
	// The file name of the child RemoteResourecS3
	ChildRSS3FIleName string `json:"childRRS3FileName,omitempty"`
	// Url used by the razee install to post data
	RazeeDashUrl string `json:"razeeDashUrl,omitempty"`
	// The url of the filesource arg that gets passed into the razeedeploy-job
	FileSourceURL string `json:"fileSourceUrl,omitempty"`
}

// RazeeDeploymentSpec defines the desired state of RazeeDeployment
// +k8s:openapi-gen=true
type RazeeDeploymentSpec struct {
	// Setting enabled to "true" will create a Razee namespace and deploy it's componenets. Set to "false" to bypass Razee installation
	Enabled bool `json:"enabled"`

	// ClusterUUID is the cluster identifier, used for installing razee.
	ClusterUUID string `json:"clusterUUID"`

	// DeploySecretName is the name of our secret where Razee
	// variables are stored.
	// +optional
	DeploySecretName *string `json:"deploySecretName,omitempty"`

	// TargetNamespace is configurable target of the razee namespace
	// This is to support legancy installs. Please do not edit.
	// +optional
	TargetNamespace *string `json:"targetNamespace,omitempty"`

	// Configuration values provided from redhat marketplace
	// These are used internally by the Operator
	// +optional
	DeployConfig *RazeeConfigurationValues `json:"deployConfig,omitempty"`

	// Location of your IBM Cloud Object Storage resources
	// Used internally by the Operator
	// +optional
	ChildUrl *string `json:"childUrl,omitempty"`
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
	// Conditions represent the latest available observations of an object's stateonfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	JobConditions *batch.JobCondition `json:"conditions,omitempty"`
	// JobState is the status of the Razee Install Job
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	JobState batch.JobStatus `json:"jobState,omitempty"`

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
}

type RazeeJobInstallStruct struct {
	// RazeeNamespace is the namespace targeted for the Razee install
	RazeeNamespace string `json:"razee_namespace"`
	// RazeeInstallURL is the url used to install the Razee resources
	RazeeInstallURL string `json:"razee_install_url"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// RazeeDeployment is the resources that deploys Razee for the Red Hat Marketplace.
// This is an internal resource not meant to be modified directly.
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=razeedeployments,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="(Internal) Razee Deployment"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Job,v1,"redhat-marketplace-operator"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ConfigMap,v1,"redhat-marketplace-operator"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Secret,v1,"redhat-marketplace-operator"`
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

// These are valid conditions of RazeeDeployment
const (

	// Reasons for install
	ReasonRazeeStartInstall                 status.ConditionReason = "StartRazeeInstall"
	ReasonWatchKeeperNonNamespacedInstalled status.ConditionReason = "FinishedWatchKeeperNonNamespaceInstall"
	ReasonWatchKeeperLimitPollInstalled     status.ConditionReason = "FinishedWatchKeeperLimitPollInstall"
	ReasonRazeeClusterMetaDataInstalled     status.ConditionReason = "FinishedRazeeClusterMetaDataInstall"
	ReasonWatchKeeperConfigInstalled        status.ConditionReason = "FinishedWatchKeeperConfigInstall"
	ReasonWatchKeeperSecretInstalled        status.ConditionReason = "FinishedWatchKeeperSecretInstall"
	ReasonCosReaderKeyInstalled             status.ConditionReason = "FinishedCosReaderKeyInstall"
	ReasonRazeeDeployJobStart               status.ConditionReason = "StartRazeeDeployJob"
	ReasonRazeeDeployJobFinished            status.ConditionReason = "FinishedRazeeDeployJob"
	ReasonParentRRS3Installed               status.ConditionReason = "FinishParentRRS3Install"
	ReasonRazeeInstallFinished              status.ConditionReason = "FinishedRazeeInstall"
)
