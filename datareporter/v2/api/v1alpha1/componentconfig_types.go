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
	"time"

	"github.com/gotidy/ptr"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
	cfg "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
)

type ApiHandlerConfig struct {
	// HandlerTimeout is the timeout for the datareporter operator api handler
	HandlerTimeout metav1.Duration `json:"handlerTimeout,omitempty"`

	// ConfirmDelivery configures the api handler
	// true: skips the EventEngine accumulator and generates 1 report with 1 event
	// The handler will wait for 200 OK for DataService delivery before returning 200 OK
	// false: enters the event into the EventEngine accumulator and generates 1 report with N events
	// The handler will return a 200 OK for DataService delivery as long as the event json is valid
	ConfirmDelivery *bool `json:"confirmDelivery,omitempty"`
}

type EventEngineConfig struct {
	// AccMemoryLimit is the event accumulator memory limit
	AccMemoryLimit resource.Quantity `json:"memoryLimit,omitempty"`

	// MaxFlushTimeout is the max time before events are flushed
	MaxFlushTimeout metav1.Duration `json:"maxFlushTimeout,omitempty"`

	// MaxEventEntries is the max entries per key allowed in the event accumulator
	MaxEventEntries int `json:"maxEventEntries,omitempty"`

	DataServiceTokenFile string `json:"dataServiceTokenFile,omitempty"`

	DataServiceCertFile string `json:"dataServiceCertFile,omitempty"`
}

type ManagerConfig struct {
	LeaderElectionID string `json:"leaderElectionId,omitempty"`

	cfg.ControllerManagerConfigurationSpec `json:",inline"`
}

// ComponentConfigStatus defines the observed state of ComponentConfig
type ComponentConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ComponentConfig is the Schema for the componentconfigs API
// The TLSConfig in ComponentConfig modifies the TLSConfig of the DataService Client
type ComponentConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ApiHandlerConfig  `json:"apiHandlerConfig,omitempty"`
	EventEngineConfig `json:"eventHandlerConfig,omitempty"`
	ManagerConfig     `json:"managerConfig,omitempty"`
	TLSConfig         `json:"tlsConfig,omitempty"`
	Status            ComponentConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentConfigList contains a list of ComponentConfig
type ComponentConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentConfig{}, &ComponentConfigList{})
}

func NewComponentConfig() *ComponentConfig {
	handlerDuration, _ := time.ParseDuration("30s")
	accMemLimit, _ := resource.ParseQuantity("50mi")
	maxFlushTimeout, _ := time.ParseDuration("300s")

	controllerManagerSpec := cfg.ControllerManagerConfigurationSpec{
		Health: cfg.ControllerHealth{
			HealthProbeBindAddress: ":8081",
		},
		Metrics: cfg.ControllerMetrics{
			BindAddress: ":8443",
		},
		Webhook: cfg.ControllerWebhook{
			Port: ptr.Int(9443),
		},
		LeaderElection: &configv1alpha1.LeaderElectionConfiguration{
			LeaderElect: ptr.Bool(true),
		},
	}

	cc := &ComponentConfig{
		ApiHandlerConfig: ApiHandlerConfig{
			HandlerTimeout: metav1.Duration{Duration: handlerDuration},
		},
		EventEngineConfig: EventEngineConfig{
			AccMemoryLimit:       accMemLimit,
			MaxFlushTimeout:      metav1.Duration{Duration: maxFlushTimeout},
			MaxEventEntries:      50,
			DataServiceTokenFile: "/etc/data-service-sa/data-service-token",
			DataServiceCertFile:  "/etc/configmaps/serving-cert-ca-bundle/service-ca.crt",
		},
		ManagerConfig: ManagerConfig{
			LeaderElectionID:                   "datareporter.marketplace.redhat.com",
			ControllerManagerConfigurationSpec: controllerManagerSpec,
		},
		TLSConfig: TLSConfig{
			CipherSuites: []string{"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
			MinVersion: "VersionTLS12",
		},
	}

	return cc
}
