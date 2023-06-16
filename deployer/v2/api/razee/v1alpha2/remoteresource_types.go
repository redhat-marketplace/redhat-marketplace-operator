// Copyright 2023 IBM Corp.
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

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RemoteResource is the Schema for the remoteresources API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=remoteresources,scope=Namespaced
// +k8s:openapi-gen=true
type RemoteResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RemoteResourceSpec   `json:"spec,omitempty"`
	Status            RemoteResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RemoteResourceList contains a list of RemoteResources
type RemoteResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteResource `json:"items"`
}

// RemoteResourceSpec defines the desired state of RemoteResource
// +kubebuilder:pruning:PreserveUnknownFields
// +k8s:openapi-gen=true
type RemoteResourceSpec struct {
	Auth           RemoteResourceAuth `json:"auth,omitempty"`
	ClusterAuth    ClusterAuth        `json:"clusterAuth,omitempty"`
	BackendService BackendService     `json:"backendService,omitempty"`
	Requests       []Request          `json:"requests,omitempty"`
}

// RemoteResourceStatus defines the observed state of RemoteResource
// +k8s:openapi-gen=true
type RemoteResourceStatus struct {
}

type ClusterAuth struct {
	ImpersonateUser string `json:"impersonateUser,omitempty"`
}

// +kubebuilder:validation:Enum=generic;s3;git
type BackendService string

type RemoteResourceAuth struct {
	Hmac *RemoteResourceHmac `json:"hmac,omitempty"`
	Iam  *RemoteResourceIam  `json:"iam,omitempty"`
}

type RemoteResourceIam struct {
	GrantType string `json:"grantType,omitempty"`
	URL       string `json:"url,omitempty"`
	// +optional
	APIKey string `json:"apiKey,omitempty"`
	// +optional
	APIKeyRef APIKeyRef `json:"apiKeyRef,omitempty"`
}

type RemoteResourceHmac struct {
	// +optional
	AccessKeyID string `json:"accessKeyId,omitempty"`
	// +optional
	AccesKeyIDRef AccesKeyIDRef `json:"accessKeyIdRef,omitempty"`
	// +optional
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
	// +optional
	SecretAccessKeyRef SecretAccessKeyRef `json:"secretAccessKeyRef,omitempty"`
}

type RemoteResourceSecretAccessKeyRef struct {
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

type RemoteResourceAccesKeyIDRef struct {
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

type RemoteResourceAPIKeyRef struct {
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

type RemoteResourceValueFrom struct {
	// +kubebuilder:validation:Required
	SecretKeyRef corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}
type URL struct {
	Format string `json:"format,omitempty"`
}

type RemoteResourceRequest struct {
	URL         string      `json:"url,omitempty"`
	URI         string      `json:"uri,omitempty"`
	Git         Git         `json:"git,omitempty"`
	Headers     Headers     `json:"headers,omitempty"`
	HeadersFrom HeadersFrom `json:"headersFrom,omitempty"`
}

type Git struct {
	Provider Provider `json:"provider,omitempty"`
	Repo     string   `json:"repo,omitempty"`
	Ref      string   `json:"ref,omitempty"`
	FilePath string   `json:"filePath,omitempty"`
	Release  string   `json:"release,omitempty"`
}

// +kubebuilder:validation:Enum=github;gitlab
type Provider string

type Headers struct {
	Headers map[string]Header `json:"headers,omitempty"`
}

type HeadersFrom struct {
	ConfigMapRef  ConfigMapRef  `json:"configMapRef,omitempty"`
	SecretMapRef  SecretMapRef  `json:"secretKeyRef,omitempty"`
	GenericMapRef GenericMapRef `json:"genericMapRef,omitempty"`
}

type ConfigMapRef struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type SecretMapRef struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type GenericMapRef struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
}

// Header allows you to provide additional information with your reques
type Header map[string]string

// Request holds requests that populate the Requests array
type Request struct {
	// Options is the configurable options for the request
	// +kubebuilder:validation:Required
	Options S3Options `json:"options,omitempty"`
	// Optional if downloading or applying a child resource fails, RemoteResource will stop execution and report error to .status. You can allow execution to continue by marking a reference as optional.
	// +optional
	Optional bool `json:"optional,omitempty"`
	// Status of the request
	// +optional
	StatusCode int `json:"statusCode,omitempty"`
	// Message of the request
	// +optional
	Message string `json:"message,omitempty"`
}

// Options holds the options object which will be passed as-is to the http request. Allows you to specify things like headers for authentication.
type S3Options struct {
	// URL of the request
	// +optional
	URL string `json:"url,omitempty"`
	// URI of the request
	// +optional
	URI string `json:"uri,omitempty"`
	// Headers of the request
	// +optional
	Headers map[string]Header `json:"headers,omitempty"`
}

// SecretAccessKeyRef holds reference information to an SecretAccessKey stored in a secret on your cluster
type SecretAccessKeyRef struct {
	// ValueFrom is the pointer to the secret key ref
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

// AccesKeyIDRef holds reference information to an AccessKeyID stored in a secret on your cluster
type AccesKeyIDRef struct {
	// ValueFrom is the pointer to the secret key ref
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

// APIKeyRef holds the location of the api key used to authenticate to a cloud object storage instance
type APIKeyRef struct {
	// ValueFrom is the pointer to the secret key ref
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom,omitempty"`
}

// ValueFrom holds source for the environment variable's value. Cannot be used if value is not empty.
type ValueFrom struct {
	// SecretKeyRef is the pointer to the secret key ref
	// +kubebuilder:validation:Required
	SecretKeyRef corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

func init() {
	SchemeBuilder.Register(&RemoteResource{}, &RemoteResourceList{})
}
