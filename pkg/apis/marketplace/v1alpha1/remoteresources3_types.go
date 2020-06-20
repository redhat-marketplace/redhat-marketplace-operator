package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

//Auth allows you authenticate to remote storage locations using either HMAC or IAM authentication schemes.
type Auth struct {
	// +optional
	Hmac *Hmac `json:"hmac,omitempty"`
	// +optional
	Iam *Iam `json:"iam,omitempty"`
}

//Hmac allows you to connect to s3 buckets using an HMAC key/id pair.
type Hmac struct {
	// AccessKeyID is a unique identifier for an AWS account and is used by AWS to look up your Secret Access Key
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	AccessKeyID string `json:"accessKeyId,omitempty"`
	// AccesKeyIDRef holds reference information to an AccessKeyID stored in a secret on your cluster
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	AccesKeyIDRef AccesKeyIDRef `json:"accessKeyIdRef,omitempty"`
	// SecretAccessKey is used by AWS to calculate a request signature. Your SecretAccessKey is a shared secret known only to you and AWS
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
	// SecretAccessKeyRef holds reference information to an SecretAccessKey stored in a secret on your cluster
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	SecretAccessKeyRef SecretAccessKeyRef `json:"secretAccessKeyRef,omitempty"`
}

//Iam Allows you to connect to s3 buckets using an IAM provider and api key.
type Iam struct {
	// ResponseType specifies which grant type your application is requesting. ResponseType for IAM will usually be "cloud_iam"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	ResponseType string `json:"responseType,omitempty"`
	// GrantType determines what authentication flow will be used to generate an access token. GrantType for IAM will usually be " "urn:ibm:params:oauth:grant-type:apikey""
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	GrantType string `json:"grantType,omitempty"`
	// URL is the auth endpoint. URL for IAM will usually be "https://iam.cloud.ibm.com/identity/token"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	URL string `json:"url,omitempty"`
	// APIKey is the API Key used to authenticate to your IBM Cloud Object Storage instance
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	APIKey string `json:"apiKey,omitempty"`
	// APIKeyRef holds reference information used to locate a secret which contains your IBM COS api key
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	APIKeyRef APIKeyRef `json:"apiKeyRef,omitempty"`
}

// SecretAccessKeyRef holds reference information to an SecretAccessKey stored in a secret on your cluster
type SecretAccessKeyRef struct {
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

// AccesKeyIDRef holds reference information to an AccessKeyID stored in a secret on your cluster
type AccesKeyIDRef struct {
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

//APIKeyRef holds the location of the api key used to authenticate to a cloud object storage instance
type APIKeyRef struct {
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

//ValueFrom holds source for the environment variable's value. Cannot be used if value is not empty.
type ValueFrom struct {
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:Required
	SecretKeyRef corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// Request holds requests that populate the Requests array
type Request struct {
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:Required
	Options Options `json:"options,omitempty"`
	// if downloading or applying a child resource fails, RemoteResource will stop execution and report error to .status. You can allow execution to continue by marking a reference as optional.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Optional bool `json:"optional,omitempty"`
}

//Options holds the options object which will be passed as-is to the http request. Allows you to specify things like headers for authentication.
type Options struct {
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	URL string `json:"url,omitempty"`
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	URI string `json:"uri,omitempty"`
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Headers map[string]Header `json:"headers,omitempty"`
}

// Header allows you to provide additional information with your request
// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
type Header map[string]string

// RemoteResourceS3Spec defines the desired state of RemoteResourceS3
// +k8s:openapi-gen=true
type RemoteResourceS3Spec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// Auth provides options to authenticate to a remote location
	Auth Auth `json:"auth,omitempty"`
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// Requests array contains information regarding the location of your remote resource
	Requests []Request `json:"requests,omitempty"`
}

// RemoteResourceS3Status defines the observed state of RemoteResourceS3
// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
// +k8s:openapi-gen=true
// +optional
// +kubebuilder:pruning:PreserveUnknownFields
type RemoteResourceS3Status struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	RazeeLogs RazeeLogs `json:"razeeLogs,omitempty"`
}

// RazeeLogs holds log output from the RRS3 controller
// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
// +optional
type RazeeLogs struct {
	Log Log `json:"error,omitempty"`
}

// Log holds a log message as <log hash>:<log message>
// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
// +optional
type Log map[string]string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RemoteResourceS3 is the Schema for the remoteresources3s API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=remoteresources3s,scope=Namespaced
type RemoteResourceS3 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteResourceS3Spec   `json:"spec,omitempty"`
	Status RemoteResourceS3Status `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RemoteResourceS3List contains a list of RemoteResourceS3
type RemoteResourceS3List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteResourceS3 `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteResourceS3{}, &RemoteResourceS3List{})
}
