/*
Copyright 2023 IBM Co..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DataReporterConfigSpec defines the desired state of DataReporterConfig.
type DataReporterConfigSpec struct {
	// +optional
	UserConfigs []UserConfig `json:"userConfig,omitempty"`
	// DataFilter to match incoming event payload against.
	// The first Selector match in the DataFilters array will be applied.
	// +optional
	DataFilters []DataFilter `json:"dataFilters,omitempty"`
	// TLSConfig specifies TLS configuration parameters for outbound https requests from the client.
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`
	// ConfirmDelivery configures the api handler. Takes priority over configuring ComponentConfig.
	// true: skips the EventEngine accumulator and generates 1 report with 1 event.
	// The handler will wait for 200 OK for DataService delivery before returning 200 OK.
	// false: enters the event into the EventEngine accumulator and generates 1 report with N events.
	// The handler will return a 200 OK for DataService delivery as long as the event json is valid.
	ConfirmDelivery *bool `json:"confirmDelivery,omitempty"`
}

// UserConfig defines additional metadata added to a specified users report.
type UserConfig struct {
	// Required.
	UserName string `json:"userName,omitempty"`
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// DataFilter defines json transformation and alternate event payload destinations based on selector criteria.
// No Selector indicates match all.
type DataFilter struct {
	// +optional
	Selector Selector `json:"selector,omitempty"`
	// +optional
	ManifestType string `json:"manifestType,omitempty"`
	// +optional
	Transformer Transformer `json:"transformer,omitempty"`
	// +optional
	AltDestinations []Destination `json:"altDestinations,omitempty"`
}

// Selector defines criteria for matching incoming event payload.
type Selector struct {
	// MatchExpressions is a list of jsonpath expressions.
	// To match the Selector, all jsonpath expressions must produce a result (AND).
	// +optional
	MatchExpressions []string `json:"matchExpressions,omitempty"`

	// MatchUsers is a list of users that the dataFilter applies to.
	// If MatchUsers is not specified, the DataFilter applies to all users.
	// +optional
	MatchUsers []string `json:"matchUsers,omitempty"`
}

// Transformer defines the type of transformer to use, and where to load the transformation configuration from.
type Transformer struct {
	// type is the transformation engine use
	// supported types: kazaam
	TransformerType string `json:"type,omitempty"`
	// configMapKeyRef refers to the transformation configuration residing in a ConfigMap
	ConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty" protobuf:"bytes,3,opt,name=configMapKeyRef"`
}

// TLSConfig refers to TLS configuration options.
type TLSConfig struct {
	// CACertsSecret refers to a list of secret keys that contains CA certificates in PEM. crypto/tls Config.RootCAs.
	// +optional
	CACerts []corev1.SecretKeySelector `json:"caCerts,omitempty"`
	// Certificates refers to a list of X509KeyPairs consisting of the client public/private key. crypto/tls Config.Certificates.
	// +optional
	Certificates []Certificate `json:"certificates,omitempty"`
	// If true, skips creation of TLSConfig with certs and creates an empty TLSConfig. crypto/tls Config.InsecureSkipVerify (Defaults to false).
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty" protobuf:"varint,4,opt,name=insecureSkipVerify"`
	// CipherSuites is a list of enabled cipher suites. crypto/tls Config.CipherSuites.
	// +optional
	CipherSuites []string `json:"cipherSuites,omitempty"`
	// MinVersion contains the minimum TLS version that is acceptable crypto/tls Config.MinVersion.
	// +optional
	MinVersion string `json:"minVersion,omitempty"`
}

// SecretKeyRef refers to a SecretKeySelector
type SecretKeyRef struct {
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// Certificate refers to the the X509KeyPair, consisting of the secrets containing the key and cert pem.
type Certificate struct {
	// ClientCert refers to the SecretKeyRef that contains the client cert PEM
	ClientCert SecretKeyRef `json:"clientCert,omitempty"`
	// ClientKey refers to the SecretKeyRef that contains the client key PEM
	ClientKey SecretKeyRef `json:"clientKey,omitempty"`
}

// Destination defines an additional endpoint to forward a transformed event payload to.
type Destination struct {
	// Transformer refers to the Transformer to apply.
	// +optional
	Transformer Transformer `json:"transformer,omitempty"`

	// URL is the destination endpoint (https://hostname:port/path).
	URL string `json:"url"`

	// URLSuffixExpr is a jsonpath expression used to parse the event. The result is appended to the destination URL.
	// https://example/path/{URLSuffixExprResult}
	// +optional
	URLSuffixExpr string `json:"urlSuffixExpr,omitempty"`

	// Sets the sources of the headers to pass to the client when calling the destination URL.
	// +optional
	Header Header `json:"header,omitempty"`

	// Sets an optional authorization endpoint to first request a token from.
	// The Authorization endpoint is called if the call to the destination URL results in a 403.
	// +optional
	Authorization Authorization `json:"authorization,omitempty"`
}

// Sources of headers to append to request.
type Header struct {
	// Sets the name of the secret that contains the headers. Secret map key/value pairs will be used for the header.
	// +optional
	Secret corev1.LocalObjectReference `json:"secret,omitempty"`
}

// Authorization defines an endpoint to request a token from.
type Authorization struct {
	// URL is the authorization endpoint (https://hostname:port/path).
	URL string `json:"url"`

	// Sets the sources of the headers to pass to the client when calling the authorization URL.
	// +optional
	Header Header `json:"header,omitempty"`

	// Sets the additional header map key to set on the Destination header ("Authorization").
	// +optional
	AuthDestHeader string `json:"authDestHeader,omitempty"`

	// AuthDestHeaderPrefix: the additional prefix map string value to set on the destHeader ("Bearer ").
	// +optional
	AuthDestHeaderPrefix string `json:"authDestHeaderPrefix,omitempty"`

	// TokenExpr is a jsonpath expression used to parse the authorization response in order to extract the token.
	// +optional
	TokenExpr string `json:"tokenExpr,omitempty"`

	// BodyData refers to a SecretKeyRef containing body data to POST to authorization endpoint, such as an api key.
	// +optional
	BodyData SecretKeyRef `json:"bodyData,omitempty"`
}

// DataReporterConfigStatus defines the observed state of DataReporterConfig
type DataReporterConfigStatus struct {
	// Conditions represent the latest available observations of an object's stateconfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DataReporterConfig is the Schema for the datareporterconfigs API
type DataReporterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataReporterConfigSpec   `json:"spec,omitempty"`
	Status DataReporterConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DataReporterConfigList contains a list of DataReporterConfig
type DataReporterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataReporterConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DataReporterConfig{}, &DataReporterConfigList{})
}

const (

	// license not accepted condition, operator will not process events
	ConditionNoLicense       status.ConditionType   = "NoLicense"
	ReasonLicenseNotAccepted status.ConditionReason = "LicenseNotAccepted"

	// problem with data upload to data service
	ConditionUploadFailure status.ConditionType   = "UploadFailed"
	ReasonUploadFailed     status.ConditionReason = "UploadFailed"

	// unable to connect to data service
	ConditionConnectionFailure status.ConditionType   = "DataServiceConnectionFailed"
	ReasonConnectionFailure    status.ConditionReason = "DataServiceConnectionFailed"

	// datafilter configuration is invalid
	ConditionDataFilterInvalid status.ConditionType   = "DataFilterInvalid"
	ReasonDataFilterInvalid    status.ConditionReason = "DataFilterInvalid"
)
