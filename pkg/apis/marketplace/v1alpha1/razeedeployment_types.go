package v1alpha1

import (
	batch "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// type RazeeConfigValues struct {
// 	RazeeDashOrgKey   string `json:"razeeDashOrgKey,omitempty"`
// 	BucketName        string `json:"bucketName,omitempty"`
// 	IbmCosUrl         string  `json:"ibmCosUrl,omitempty"`
// 	ChildRRS3FileName string  `json:"childRRS3FileName,omitempty"`
// 	IbmCosReaderKey   string  `json:"ibmCosReaderKey,omitempty"`
// 	RazeeDashUrl      string  `json:"razeeDashUrl,omitempty"`
// 	FileSourceUrl     string  `json:"fileSourceUrl,omitempty"`
// 	IbmCosFullUrl     string  `json:"ibmCosFullUrl,omitempty"`
// }

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RazeeDeploymentSpec defines the desired state of RazeeDeployment
type RazeeDeploymentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// Setting enabled to "true" will create a Razee namespace and deploy it's componenets. Set to "false" to bypass Razee installation
	Enabled          bool    `json:"enabled"`
	ClusterUUID      string  `json:"clusterUUID"`
	DeploySecretName *string `json:"deploySecretName,omitempty"`
	DeploySecretValues map[string]string `json:"deploySecretValues,omitempty"`
	IbmCosFullUrl *string `json:"ibmCosFullUrl,omitempty"`
}

// RazeeDeploymentStatus defines the observed state of RazeeDeployment
type RazeeDeploymentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Conditions                   *batch.JobCondition `json:"conditions,omitempty"`
	JobState                     batch.JobStatus     `json:"jobState,omitempty"`
	MissingDeploySecretValues      *[]string           `json:"missingDeploySecretValues,omitempty"`
	RazeePrerequisitesCreated    *[]string           `json:"razeePrerequisitesCreated,omitempty"`
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
