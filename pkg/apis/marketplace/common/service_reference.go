package common

import (
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

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
