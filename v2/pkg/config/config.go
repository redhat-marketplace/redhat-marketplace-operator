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
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/caarlos0/env/v6"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"k8s.io/client-go/discovery"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var global *OperatorConfig
var globalMutex = sync.RWMutex{}
var log = logf.Log.WithName("operator_config")
var defaultOperatorConfigFile = "/config/config.yaml"

// OperatorConfig is the configuration for the operator
type OperatorConfig struct {
	DeployedNamespace string                 `env:"POD_NAMESPACE"`
	ReportController  ReportControllerConfig `json:"reportController,omitempty"`
	RelatedImages     `json:"relatedImages,omitempty"`
	ResourcesLimits   `json:"resourceLimits,omitempty"`
	Features
	Marketplace `json:"marketplace,omitempty"`
	*Infrastructure
	OLMInformation
}

// RelatedImages stores relatedimages for the operator
type RelatedImages struct {
	Reporter                    string `env:"RELATED_IMAGE_REPORTER" envDefault:"reporter:latest" json:"reporter,omitempty"`
	KubeRbacProxy               string `env:"RELATED_IMAGE_KUBE_RBAC_PROXY" envDefault:"registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.5" json:"kubeRbacProxy,omitempty"`
	MetricState                 string `env:"RELATED_IMAGE_METRIC_STATE" envDefault:"metric-state:latest" json:"metricState,omitempty"`
	AuthChecker                 string `env:"RELATED_IMAGE_AUTHCHECK" envDefault:"authcheck:latest" json:"authChecker,omitempty"`
	Prometheus                  string `env:"RELATED_IMAGE_PROMETHEUS" envDefault:"registry.redhat.io/openshift4/ose-prometheus:latest" json:"prometheus,omitempty"`
	PrometheusOperator          string `env:"RELATED_IMAGE_PROMETHEUS_OPERATOR" envDefault:"registry.redhat.io/openshift4/ose-prometheus-operator:latest" json:"prometheusOperator,omitempty"`
	ConfigMapReloader           string `env:"RELATED_IMAGE_CONFIGMAP_RELOADER" envDefault:"registry.redhat.io/openshift4/ose-configmap-reloader:latest" json:"configMapReloader,omitempty"`
	PrometheusConfigMapReloader string `env:"RELATED_IMAGE_PROMETHEUS_CONFIGMAP_RELOADER" envDefault:"registry.redhat.io/openshift4/ose-prometheus-config-reloader:latest" json:"prometheusConfigMapReloader,omitempty"`
	OAuthProxy                  string `env:"RELATED_IMAGE_OAUTH_PROXY" envDefault:"registry.redhat.io/openshift4/ose-oauth-proxy:latest" json:"oAuthProxy,omitempty"`
	RemoteResourceS3            string `env:"RELATED_IMAGE_RHM_RRS3_DEPLOYMENT" envDefault:"quay.io/razee/remoteresources3:0.6.2" json:"remoteResourcesS3,omitempty"`
	WatchKeeper                 string `env:"RELATED_IMAGE_RHM_WATCH_KEEPER_DEPLOYMENT" envDefault:"quay.io/razee/watch-keeper:0.6.6" json:"watchKeeper,omitempty"`
}

// ResourcesLimits stores resources.limits overrides for operand containers
type ResourcesLimits struct {
	AuthcheckCPU                        string `env:"RESOURCE_LIMIT_CPU_AUTHCHECK" envDefault:"20m" json:"authcheckCPU,omitempty"`
	AuthcheckMemory                     string `env:"RESOURCE_LIMIT_MEMORY_AUTHCHECK" envDefault:"40Mi" json:"authcheckMemory,omitempty"`
	PrometheusConfigReloaderCPU         string `env:"RESOURCE_LIMIT_CPU_PROMETHEUS_CONFIG_RELOADER" envDefault:"100m" json:"prometheusConfigReloaderCPU,omitempty"`
	PrometheusConfigReloaderMemory      string `env:"RESOURCE_LIMIT_MEMORY_PROMETHEUS_CONFIG_RELOADER" envDefault:"25Mi" json:"prometheusConfigReloaderMemory,omitempty"`
	RulesConfigMapReloaderCPU           string `env:"RESOURCE_LIMIT_CPU_RULES_CONFIGMAP_RELOADER" envDefault:"100m" json:"rulesConfigMapReloaderCPU,omitempty"`
	RulesConfigMapReloaderMemory        string `env:"RESOURCE_LIMIT_MEMORY_RULES_CONFIGMAP_RELOADER" envDefault:"25Mi" json:"rulesConfigMapReloaderMemory,omitempty"`
	RHMRemoteResources3ControllerCPU    string `env:"RESOURCE_LIMIT_CPU_RHM_REMOTE_RESOURCES3_CONTROLLER" envDefault:"100m" json:"rhmRemoteResources3ControllerCPU,omitempty"`
	RHMRemoteResources3ControllerMemory string `env:"RESOURCE_LIMIT_MEMORY_RHM_REMOTE_RESOURCES3_CONTROLLER" envDefault:"200Mi" json:"rhmRemoteResources3ControllerMemory,omitempty"`
	WatchKeeperCPU                      string `env:"RESOURCE_LIMIT_CPU_WATCH_KEEPER" envDefault:"400m" json:"watchKeeperCPU,omitempty"`
	WatchKeeperMemory                   string `env:"RESOURCE_LIMIT_MEMORY_WATCH_KEEPER" envDefault:"500Mi" json:"watchKeeperMemory,omitempty"`
}

// Features store feature flags
type Features struct {
	IBMCatalog bool `env:"FEATURE_IBMCATALOG" envDefault:"true"`
}

// Marketplace configuration
type Marketplace struct {
	URL            string `env:"MARKETPLACE_URL" envDefault:"https://marketplace.redhat.com" json:"url,omitempty"`
	InsecureClient bool   `env:"MARKETPLACE_HTTP_INSECURE_MODE" envDefault:"false" json:"insecureClient,omitempty"`
}

// ReportConfig stores some changeable information for creating a report
type ReportControllerConfig struct {
	RetryTime  time.Duration `env:"REPORT_RETRY_TIME_DURATION" envDefault:"6h" json:"retryTime,omitempty"`
	RetryLimit *int32        `env:"REPORT_RETRY_LIMIT" json:"retryLimit,omitempty"`
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

// ProvideConfig gets the config from env vars
func ProvideConfig() (OperatorConfig, error) {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	if global == nil {
		cfg := OperatorConfig{}
		err := env.Parse(&cfg)
		if err != nil {
			return cfg, err
		}

		// If the ConfigMap is mounted, use values with priority over the envVar
		filename := defaultOperatorConfigFile
		ocm, ok := os.LookupEnv("OPERATOR_CONFIGMAP")
		if ok {
			filename = ocm
		}

		err = GetConfigFromConfigMap(&cfg, filename)
		if err != nil {
			return cfg, err
		}

		cfg.Infrastructure = &Infrastructure{}
		global = &cfg
	}

	return *global, nil
}

// ProvideInfrastructureAwareConfig loads Operator Config with Infrastructure information
func ProvideInfrastructureAwareConfig(
	c rhmclient.SimpleClient,
	dc *discovery.DiscoveryClient,
) (OperatorConfig, error) {
	cfg := OperatorConfig{}
	inf, err := NewInfrastructure(c, dc)

	if err != nil {
		return cfg, err
	}

	cfg.Infrastructure = inf

	err = env.Parse(&cfg)
	if err != nil {
		return cfg, err
	}

	// If the ConfigMap is mounted, use values with priority over the envVar
	filename := defaultOperatorConfigFile
	ocm, ok := os.LookupEnv("OPERATOR_CONFIGMAP")
	if ok {
		filename = ocm
	}

	err = GetConfigFromConfigMap(&cfg, filename)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

var GetConfig = ProvideConfig

// Get the Resource Limits from an optionally mounted ConfigMap
func GetConfigFromConfigMap(cfg *OperatorConfig, filename string) error {
	// Ignore if not mounted
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return nil
	}

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(file, cfg)
	if err != nil {
		return err
	}

	return nil
}
