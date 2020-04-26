package v1alpha1

import (
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Configuration values used by Razee to communicate with the Red Hat Marketplace backend
type RazeeConfigurationValues struct {
	// BucketName is the name of the bucket in Cloud Object Storage and correlates to your accountID
	BucketName string `json:"BUCKET_NAME,omitempty"`
	// The file name of the child RemoteResourecS3
	ChildRSSFIleName string `json:"CHILD_RRS3_YAML_FILENAME,omitempty"`
	// The url of the filesource arg that gets passed into the razeedeploy-job
	FileSourceURL string `json:"FILE_SOURCE_URL,omitempty"`
	// Api key used to access the bucket IBM COS
	IbmCosReaderKey *corev1.SecretKeySelector `json:"IBM_COS_READER_KEY,omitempty"`
	// Base url for the instance of IBM COS
	IbmCosURL string `json:"IBM_COS_URL,omitempty"`
	// Key used to identify a particular razee instance
	RazeeDashOrgKey *corev1.SecretKeySelector `json:"RAZEE_DASH_ORG_KEY,omitempty"`
	// Url used by the razee install to post data
	RazeeDashUrl string `json:"RAZEE_DASH_URL,omitempty"`
}

// RazeeDeploymentSpec defines the desired state of RazeeDeployment
type RazeeDeploymentSpec struct {
	// Enabled flag stops razee from installing
	Enabled          bool    `json:"enabled"`

	// ClusterUUID is the cluster identifier, used for installing razee.
	ClusterUUID      string  `json:"clusterUUID"`

	// DeploySecretName is the name of our secret where Razee
	// variables are stored.
	// +optional
	DeploySecretName *string `json:"deploySecretName,omitempty"`

	// TargetNamespace is configurable target of the razee namespace
	// this is to support legancy installs. Please do not edit.
	// +optional
	TargetNamespace  *string  `json:"targetNamespace,omitEmpty"`

	// Configuration values provided from redhat marketplace
	// These are used internally by the Operator
	// +optional
	DeployConfig     *RazeeConfigurationValues `json:"deployConfig,omitempty"`

	// Location of your IBM Cloud Object Storage resources
	// Used internally by the Operator
	// +optional
	ChildUrl         *string                   `json:"childUrl,omitempty"`
}

// RazeeDeploymentStatus defines the observed state of RazeeDeployment
type RazeeDeploymentStatus struct {
	// Conditions represent the latest available observations of an object's stateonfig
	Conditions *batch.JobCondition `json:"conditions,omitempty"`
	// JobState is the status of the Razee Install Job
	JobState batch.JobStatus `json:"jobState,omitempty"`
	// MissingDeploySecretValues validates the secret provided has all the correct fields
	MissingDeploySecretValues []string    `json:"missingDeploySecretValues,omitempty"`
	// LocalSecretVarsPopulated informs if the correct local variables are correct set.
	// LocalSecretVarsPopulated *bool `json:"localSecretVarsPopulated,omitempty"`
	// RazeePrerequestesCreated is the list of configmaps and secrets required to be installed
	RazeePrerequisitesCreated []string `json:"razeePrerequisitesCreated,omitempty"`
	// RedHatMarketplaceSecretFound is the status of finding the secret in the cluster
	// RedHatMarketplaceSecretFound *bool `json:"redHatMarketplaceSecretFound,omitempty"`
	// RazeeJobInstall contains information regarding the install job so it can be removed
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
