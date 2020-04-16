package v1alpha1

import (
	batch "k8s.io/api/batch/v1"
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
	IbmCosReaderKey string `json:"IBM_COS_READER_KEY,omitempty"`
	// Base url for the instance of IBM COS 
	IbmCosURL string `json:"IBM_COS_URL,omitempty"`
	// Key used to identify a particular razee instance
	RazeeDashOrgKey string `json:"RAZEE_DASH_ORG_KEY,omitempty"`
	// Url used by the razee install to post data
	RazeeDashUrl string `json:"RAZEE_DASH_URL,omitempty"`
  }


// RazeeDeploymentSpec defines the desired state of RazeeDeployment
type RazeeDeploymentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// Setting enabled to "true" will create a Razee namespace and deploy it's componenets. Set to "false" to bypass Razee installation
	Enabled            bool              `json:"enabled"`
	ClusterUUID        string            `json:"clusterUUID"`
	DeploySecretName   *string           `json:"deploySecretName,omitempty"`
	DeployConfig *RazeeConfigurationValues `json:"deployConfig,omitempty"`
	ChildUrl           *string           `json:"childUrl,omitempty"`
}

// RazeeDeploymentStatus defines the observed state of RazeeDeployment
type RazeeDeploymentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Conditions                *batch.JobCondition    `json:"conditions,omitempty"`
	JobState                  batch.JobStatus        `json:"jobState,omitempty"`
	MissingDeploySecretValues []string               `json:"missingDeploySecretValues,omitempty"`
	RazeePrerequisitesCreated []string               `json:"razeePrerequisitesCreated,omitempty"`
	RazeeJobInstall           *RazeeJobInstallStruct `json:"razee_job_install,omitempty"`
}

type RazeeJobInstallStruct struct {
	RazeeNamespace  string `json:"razee_namespace"`
	RazeeInstallURL string `json:"razee_install_url"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RazeeDeployment is the Schema for the razeedeployments API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=razeedeployments,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="(Internal) Razee Deployment"
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
