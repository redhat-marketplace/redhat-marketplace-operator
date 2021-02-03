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

package common

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +kubebuilder:object:generate:=true
type ServiceReference struct {
	// Namespace of the job
	// Required
	Namespace string `json:"namespace"`

	// Name of the job
	// Required
	Name string `json:"name"`

	// Port name is the name of the part to select
	// Required
	TargetPort intstr.IntOrString `json:"targetPort"`

	// TLS configuration to use when scraping the endpoint
	// Optional
	TLSConfig *monitoringv1.TLSConfig `json:"tlsConfig,omitempty"`

	// File to read bearer token for scraping targets.
	BearerTokenFile string `json:"bearerTokenFile,omitempty"`
	// Secret to mount to read bearer token for scraping targets. The secret
	// needs to be in the same namespace as the service monitor and accessible by
	// the Prometheus Operator.
	BearerTokenSecret corev1.SecretKeySelector `json:"bearerTokenSecret,omitempty"`

	// BasicAuth allow an endpoint to authenticate over basic authentication
	// Optional
	BasicAuth *monitoringv1.TLSConfig `json:"basicAuth,omitempty"`
}

type ServiceMonitorReference struct {
	// Namespace of the job
	// Required
	Namespace string `json:"namespace"`

	// Name of the job
	// Required
	Name string `json:"name"`
}
