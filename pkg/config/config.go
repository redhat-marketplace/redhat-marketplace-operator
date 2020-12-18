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
	"sync"

	"github.com/caarlos0/env/v6"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var global *OperatorConfig
var globalMutex = sync.RWMutex{}
var log = logf.Log.WithName("operator_config")

// OperatorConfig is the configuration for the operator
type OperatorConfig struct {
	RelatedImages RelatedImages
	Features      Features
	Marketplace   Marketplace
	*Infrastructure
}

// RelatedImages stores relatedimages for the operator
type RelatedImages struct {
	Reporter                    string `env:"RELATED_IMAGE_REPORTER" envDefault:"reporter:latest"`
	KubeRbacProxy               string `env:"RELATED_IMAGE_KUBE_RBAC_PROXY" envDefault:"registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.5"`
	MetricState                 string `env:"RELATED_IMAGE_METRIC_STATE" envDefault:"metric-state:latest"`
	AuthChecker                 string `env:"RELATED_IMAGE_AUTHCHECK" envDefault:"authcheck:latest"`
	Prometheus                  string `env:"RELATED_IMAGE_PROMETHEUS" envDefault:"registry.redhat.io/openshift4/ose-prometheus:latest"`
	PrometheusOperator          string `env:"RELATED_IMAGE_PROMETHEUS_OPERATOR" envDefault:"registry.redhat.io/openshift4/ose-prometheus-operator:latest"`
	ConfigMapReloader           string `env:"RELATED_IMAGE_CONFIGMAP_RELOADER" envDefault:"registry.redhat.io/openshift4/ose-configmap-reloader:latest"`
	PrometheusConfigMapReloader string `env:"RELATED_IMAGE_PROMETHEUS_CONFIGMAP_RELOADER" envDefault:"registry.redhat.io/openshift4/ose-prometheus-config-reloader:latest"`
	OAuthProxy                  string `env:"RELATED_IMAGE_OAUTH_PROXY" envDefault:"registry.redhat.io/openshift4/ose-oauth-proxy:latest"`
	RemoteResourceS3            string `env:"RELATED_IMAGE_RHM_RRS3_DEPLOYMENT" envDefault:"quay.io/razee/remoteresources3:0.6.2"`
	WatchKeeper                 string `env:"RELATED_IMAGE_RHM_WATCH_KEEPER_DEPLOYMENT" envDefault:"quay.io/razee/watch-keeper:0.6.6"`
}

// Features store feature flags
type Features struct {
	IBMCatalog bool `env:"FEATURE_IBMCATALOG" envDefault:"true"`
}

// Marketplace configuration
type Marketplace struct {
	URL            string `env:"MARKETPLACE_URL" envDefault:"https://marketplace.redhat.com"`
	InsecureClient bool   `env:"MARKETPLACE_HTTP_INSECURE_MODE" envDefault:"false"`
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
		global = &cfg
	}

	return *global, nil
}

func ProvideInfrastructureAwareConfig(c client.Client, dc *discovery.DiscoveryClient) (OperatorConfig, error) {
	cfg := OperatorConfig{}
	inf, err := LoadInfrastructure(c, dc)
	if err != nil {
		return cfg, err
	}
	cfg.Infrastructure = inf

	err = env.Parse(&cfg)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

var GetConfig = ProvideConfig
