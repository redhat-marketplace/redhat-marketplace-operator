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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cfg "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
)

type ApiHandlerConfig struct {
	// HandlerTimeout is the timeout for the datareporter operator api handler
	HandlerTimeout metav1.Duration `json:"handlerTimeout,omitempty"`
}

type EventEngineConfig struct {
	// AccMemoryLimit is the event accumulator memory limit
	AccMemoryLimit string `json:"memoryLimit,omitempty"`

	// MaxFlushTimeout is the max time before events are flushed
	MaxFlushTimeout metav1.Duration `json:"maxFlushTimeout,omitempty"`

	// MaxEventEntries is the max entries per key allowed in the event accumulator
	MaxEventEntries int `json:"maxEventEntries,omitempty"`
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
type ComponentConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ApiHandlerConfig  `json:"apiHandlerConfig,omitempty"`
	EventEngineConfig `json:"eventHandlerConfig,omitempty"`
	ManagerConfig     `json:"managerConfig,omitempty"`
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
