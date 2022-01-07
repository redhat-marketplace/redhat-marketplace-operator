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
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//Auth allows you authenticate to remote storage locations using either HMAC or IAM authentication schemes.
type Auth struct {
	// Hmac is the credentials to access the storage location.
	// +optional
	Hmac *Hmac `json:"hmac,omitempty"`
	// Iam is the credentials for Iam auth.
	// +optional
	Iam *Iam `json:"iam,omitempty"`
}

//Hmac allows you to connect to s3 buckets using an HMAC key/id pair.
type Hmac struct {
	// AccessKeyID is a unique identifier for an AWS account and is used by AWS to look up your Secret Access Key
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	AccessKeyID string `json:"accessKeyId,omitempty"`
	// AccessKeyIDRef holds reference information to an AccessKeyID stored in a secret on your cluster
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
	// ValueFrom is the pointer to the secret key ref
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

// AccesKeyIDRef holds reference information to an AccessKeyID stored in a secret on your cluster
type AccesKeyIDRef struct {
	// ValueFrom is the pointer to the secret key ref
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

//APIKeyRef holds the location of the api key used to authenticate to a cloud object storage instance
type APIKeyRef struct {
	// ValueFrom is the pointer to the secret key ref
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

//ValueFrom holds source for the environment variable's value. Cannot be used if value is not empty.
type ValueFrom struct {
	// SecretKeyRef is the pointer to the secret key ref
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:Required
	SecretKeyRef corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// Request holds requests that populate the Requests array
type Request struct {
	// Options is the configurable options for the request
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:Required
	Options S3Options `json:"options,omitempty"`
	// Optional if downloading or applying a child resource fails, RemoteResource will stop execution and report error to .status. You can allow execution to continue by marking a reference as optional.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Optional bool `json:"optional,omitempty"`
	// Status of the request
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	StatusCode int `json:"statusCode,omitempty"`
	// Message of the request
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Message string `json:"message,omitempty"`
	// Hash is the hash body value of the request
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Hash string `json:"hash,omitempty"`
}

//Options holds the options object which will be passed as-is to the http request. Allows you to specify things like headers for authentication.
type S3Options struct {
	// URL of the request
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	URL string `json:"url,omitempty"`
	// URI of the request
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	URI string `json:"uri,omitempty"`
	// Headers of the request
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Headers map[string]Header `json:"headers,omitempty"`
}

// Header allows you to provide additional information with your request
// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
type Header map[string]string

// RemoteResourceS3Spec defines the desired state of RemoteResourceS3
// +k8s:openapi-gen=true
// +kubebuilder:pruning:PreserveUnknownFields
type RemoteResourceS3Spec struct {
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// Auth provides options to authenticate to a remote location
	Auth Auth `json:"auth,omitempty"`
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// Requests array contains information regarding the location of your remote resource
	Requests []Request `json:"requests,omitempty"`
}

// RemoteResourceS3Status defines the observed state of RemoteResourceS3
// +k8s:openapi-gen=true
// +optional
// +kubebuilder:pruning:PreserveUnknownFields
type RemoteResourceS3Status struct {
	// Touched is if the status has been touched
	Touched *bool `json:"touched,omitempty"`

	// Conditions represent the latest available observations of an object's state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`
}

// RemoteResourceS3 is the Schema for the remoteresources3s API
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=remoteresources3s,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="(Internal)RemoteResourceS3"
type RemoteResourceS3 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteResourceS3Spec   `json:"spec,omitempty"`
	Status RemoteResourceS3Status `json:"status,omitempty"`
}

// These are valid conditions of a job.
const (
	// ResourceInstallError means the RemoteResourceS3 controller has a bad status (can not apply resources)
	ResourceInstallError status.ConditionType = "ResourceInstallError"

	// Reasons for install
	FailedRequest status.ConditionReason = "FailedRequest"
	NoBadRequest  status.ConditionReason = "NoBadRequest"
)

// +kubebuilder:object:root=true

// RemoteResourceS3List contains a list of RemoteResourceS3
type RemoteResourceS3List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteResourceS3 `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteResourceS3{}, &RemoteResourceS3List{})
}
