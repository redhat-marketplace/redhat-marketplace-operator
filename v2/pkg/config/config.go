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

package config

import (
	"bytes"
	"reflect"
	"strconv"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/caarlos0/env/v6"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var global *OperatorConfig
var globalMutex = sync.RWMutex{}
var log = logf.Log.WithName("operator_config")

// OperatorConfig is the configuration for the operator
type OperatorConfig struct {
	DeployedNamespace string `env:"POD_NAMESPACE"`
	DeployedPodName   string `env:"POD_NAME"`
	ControllerValues  ControllerValues
	ReportController  ReportControllerConfig
	RelatedImages
	OSRelatedImages
	Features
	Marketplace
	*Infrastructure
	OLMInformation
	MeterBaseValues
	Config EnvConfig `env:"CONFIG"`
}

// ENVCONFIG is a map of containerName to a corev1 resource requirements
// intended to set children
type EnvConfig struct {
	Features         *Features               `json:"features,omitempty"`
	Controller       *ControllerValues       `json:"controller,omitempty"`
	ReportController *ReportControllerConfig `json:"reportController,omitempty"`
	MeterController  *MeterBaseValues        `json:"meterController,omitempty"`
	Resources        *Resources              `json:"resources,omitempty"`
}

type Resources struct {
	Containers map[string]corev1.ResourceRequirements `json:"containers"`
}

// RelatedImages stores relatedimages for the operator
type RelatedImages struct {
	Reporter                    string `env:"RELATED_IMAGE_REPORTER" envDefault:"reporter:latest"`
	KubeRbacProxy               string `env:"RELATED_IMAGE_KUBE_RBAC_PROXY" envDefault:"registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.8"`
	MetricState                 string `env:"RELATED_IMAGE_METRIC_STATE" envDefault:"metric-state:latest"`
	AuthChecker                 string `env:"RELATED_IMAGE_AUTHCHECK" envDefault:"authcheck:latest"`
	DQLite                      string `env:"RELATED_IMAGE_DQLITE" envDefault:"dqlite:latest"`
	Prometheus                  string `env:"RELATED_IMAGE_PROMETHEUS" envDefault:"registry.redhat.io/openshift4/ose-prometheus:latest"`
	PrometheusOperator          string `env:"RELATED_IMAGE_PROMETHEUS_OPERATOR" envDefault:"registry.redhat.io/openshift4/ose-prometheus-operator:latest"`
	ConfigMapReloader           string `env:"RELATED_IMAGE_CONFIGMAP_RELOADER" envDefault:"registry.redhat.io/openshift4/ose-configmap-reloader:latest"`
	PrometheusConfigMapReloader string `env:"RELATED_IMAGE_PROMETHEUS_CONFIGMAP_RELOADER" envDefault:"registry.redhat.io/openshift4/ose-prometheus-config-reloader:latest"`
	OAuthProxy                  string `env:"RELATED_IMAGE_OAUTH_PROXY" envDefault:"registry.redhat.io/openshift4/ose-oauth-proxy:latest"`
	RemoteResourceS3            string `env:"RELATED_IMAGE_RHM_RRS3_DEPLOYMENT" envDefault:"quay.io/razee/remoteresources3:0.6.2"`
	WatchKeeper                 string `env:"RELATED_IMAGE_RHM_WATCH_KEEPER_DEPLOYMENT" envDefault:"quay.io/razee/watch-keeper:0.6.6"`
}

// OSRelatedImages stores open source related images for the operator
type OSRelatedImages struct {
	Reporter                    string `env:"RELATED_IMAGE_REPORTER" envDefault:"reporter:latest"`
	KubeRbacProxy               string `env:"OS_IMAGE_KUBE_RBAC_PROXY" envDefault:"quay.io/coreos/kube-rbac-proxy:v0.5.0"`
	MetricState                 string `env:"RELATED_IMAGE_METRIC_STATE" envDefault:"metric-state:latest"`
	AuthChecker                 string `env:"RELATED_IMAGE_AUTHCHECK" envDefault:"authcheck:latest"`
	DQLite                      string `env:"RELATED_IMAGE_DQLITE" envDefault:"dqlite:latest"`
	Prometheus                  string `env:"OS_IMAGE_PROMETHEUS" envDefault:"quay.io/prometheus/prometheus:v2.24.0"`
	PrometheusOperator          string `env:"OS_IMAGE_PROMETHEUS_OPERATOR" envDefault:"quay.io/coreos/prometheus-operator:v0.42.1"`
	ConfigMapReloader           string `env:"OS_IMAGE_CONFIGMAP_RELOADER" envDefault:"quay.io/coreos/configmap-reload:v0.0.1"`
	PrometheusConfigMapReloader string `env:"OS_IMAGE_PROMETHEUS_CONFIGMAP_RELOADER" envDefault:"quay.io/coreos/prometheus-config-reloader:v0.42.1"`
	OAuthProxy                  string `env:"OS_IMAGE_OAUTH_PROXY" envDefault:"quay.io/oauth2-proxy/oauth2-proxy:v6.1.1"`
	RemoteResourceS3            string `env:"RELATED_IMAGE_RHM_RRS3_DEPLOYMENT" envDefault:"quay.io/razee/remoteresources3:0.6.2"`
	WatchKeeper                 string `env:"RELATED_IMAGE_RHM_WATCH_KEEPER_DEPLOYMENT" envDefault:"quay.io/razee/watch-keeper:0.6.6"`
}

// Features store feature flags
type Features struct {
	IBMCatalog bool `env:"FEATURE_IBMCATALOG" envDefault:"true" json:"IBMCatalog"`
}

// Marketplace configuration
type Marketplace struct {
	URL            string `env:"MARKETPLACE_URL" envDefault:"" json:"url"`
	InsecureClient bool   `env:"MARKETPLACE_HTTP_INSECURE_MODE" envDefault:"false" json:"insecureClient"`
}

type ControllerValues struct {
	DeploymentNamespace           string        `env:"POD_NAMESPACE" envDefault:"openshift-redhat-marketplace" json:"deploymentNamespace"`
	MeterDefControllerRequeueRate time.Duration `env:"METER_DEF_CONTROLLER_REQUEUE_RATE" envDefault:"1h" json:"meterDefControllerRequeueRate"`
}

type MeterBaseValues struct {
	TransitionTime time.Duration `env:"METERBASE_TRANSITION_TIME" envDefault:"72h" json:"transitionTime"`
}

// ReportConfig stores some changeable information for creating a report
type ReportControllerConfig struct {
	RetryTime             time.Duration `env:"REPORT_RETRY_TIME_DURATION" envDefault:"6h"`
	RetryLimit            *int32        `env:"REPORT_RETRY_LIMIT"`
	PollTime              time.Duration `env:"REPORT_POLL_TIME_DURATION" envDefault:"1h"`
	UploadTargetsOverride []string      `env:"UPLOADTARGETSOVERRIDE" envSeparator:","`
	ReporterSchema        string        `env:"REPORTERSCHEMA"`
}

type OLMInformation struct {
	OwnerName      string `env:"OLM_OWNER_NAME"`
	OwnerNamespace string `env:"OLM_OWNER_NAMESPACE"`
	OwnerKind      string `env:"OLM_OWNER_KIND"`
}

func reset() {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	global = nil
}

var customUnmarshalers = map[reflect.Type]env.ParserFunc{
	reflect.TypeOf(EnvConfig{}): func(text string) (interface{}, error) {
		envConfig := EnvConfig{}
		err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(text)), 100).Decode(&envConfig)
		if err != nil {
			return nil, err
		}
		return envConfig, nil
	},
}

// ProvideConfig gets the config from env vars
func ProvideConfig() (*OperatorConfig, error) {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	if global == nil {
		cfg := OperatorConfig{}
		err := env.ParseWithFuncs(&cfg, customUnmarshalers)
		if err != nil {
			return nil, err
		}

		cfg.Infrastructure = &Infrastructure{}
		global = &cfg
	}

	return global, nil
}

// ProvideInfrastructureAwareConfig loads Operator Config with Infrastructure information
func ProvideInfrastructureAwareConfig(
	c rhmclient.SimpleClient,
	dc *discovery.DiscoveryClient,
) (*OperatorConfig, error) {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	if global == nil {
		cfg := &OperatorConfig{}
		inf, err := NewInfrastructure(c, dc)

		if err != nil {
			return nil, err
		}

		cfg.Infrastructure = inf

		err = env.ParseWithFuncs(cfg, customUnmarshalers)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse config")
		}

		// Use OCP version related images on Openshift 4.6+ instead of default v4.5 images
		if inf.HasOpenshift() && inf.OpenshiftParsedVersion().GTE(utils.ParsedVersion460) {
			// version 4.8 is latest one tested with RHMP operator, use it on newer releases than 4.8
			ocpTag := "4.8"

			if inf.OpenshiftParsedVersion().LT(utils.ParsedVersion480) {
				// on version 4.6 and 4.7 use images for given OCP release
				ocpVersion := inf.OpenshiftParsedVersion()
				ocpTag = strconv.FormatUint(ocpVersion.Major, 10) + "." + strconv.FormatUint(ocpVersion.Minor, 10)
			}

			if cfg.RelatedImages.Prometheus == "registry.redhat.io/openshift4/ose-prometheus:v4.5" {
				cfg.RelatedImages.Prometheus = "registry.redhat.io/openshift4/ose-prometheus:v" + ocpTag
			}
			if cfg.RelatedImages.PrometheusOperator == "registry.redhat.io/openshift4/ose-prometheus-operator:v4.5" {
				cfg.RelatedImages.PrometheusOperator = "registry.redhat.io/openshift4/ose-prometheus-operator:v" + ocpTag
			}
			if cfg.RelatedImages.OAuthProxy == "registry.redhat.io/openshift4/ose-oauth-proxy:v4.5" {
				cfg.RelatedImages.OAuthProxy = "registry.redhat.io/openshift4/ose-oauth-proxy:v" + ocpTag
			}
			if cfg.RelatedImages.ConfigMapReloader == "registry.redhat.io/openshift4/ose-configmap-reloader:v4.5" {
				cfg.RelatedImages.ConfigMapReloader = "registry.redhat.io/openshift4/ose-configmap-reloader:v" + ocpTag
			}
			if cfg.RelatedImages.PrometheusConfigMapReloader == "registry.redhat.io/openshift4/ose-prometheus-config-reloader:v4.5" {
				cfg.RelatedImages.PrometheusConfigMapReloader = "registry.redhat.io/openshift4/ose-prometheus-config-reloader:v" + ocpTag
			}
			if cfg.RelatedImages.KubeRbacProxy == "registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.5" {
				cfg.RelatedImages.KubeRbacProxy = "registry.redhat.io/openshift4/ose-kube-rbac-proxy:v" + ocpTag
			}
		}

		if !inf.HasOpenshift() {
			cfg.RelatedImages = RelatedImages(cfg.OSRelatedImages)
		}

		global = cfg
	}

	return global, nil
}

var GetConfig = ProvideConfig
