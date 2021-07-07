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
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/caarlos0/env/v6"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
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
	IsAirGap bool `env:"IS_AIRGAP" envDefault:"false"`
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

// OSRelatedImages stores open source related images for the operator
type OSRelatedImages struct {
	Reporter                    string `env:"RELATED_IMAGE_REPORTER" envDefault:"reporter:latest"`
	KubeRbacProxy               string `env:"OS_IMAGE_KUBE_RBAC_PROXY" envDefault:"quay.io/coreos/kube-rbac-proxy:v0.5.0"`
	MetricState                 string `env:"RELATED_IMAGE_METRIC_STATE" envDefault:"metric-state:latest"`
	AuthChecker                 string `env:"RELATED_IMAGE_AUTHCHECK" envDefault:"authcheck:latest"`
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
	IBMCatalog bool `env:"FEATURE_IBMCATALOG" envDefault:"true"`
}

// Marketplace configuration
type Marketplace struct {
	URL            string `env:"MARKETPLACE_URL" envDefault:""`
	InsecureClient bool   `env:"MARKETPLACE_HTTP_INSECURE_MODE" envDefault:"false"`
}

type ControllerValues struct {
	DeploymentNamespace           string        `env:"POD_NAMESPACE" envDefault:"openshift-redhat-marketplace"`
	MeterDefControllerRequeueRate time.Duration `env:"METER_DEF_CONTROLLER_REQUEUE_RATE" envDefault:"1h"`
}

// ReportConfig stores some changeable information for creating a report
type ReportControllerConfig struct {
	RetryTime  time.Duration `env:"REPORT_RETRY_TIME_DURATION" envDefault:"6h"`
	RetryLimit *int32        `env:"REPORT_RETRY_LIMIT"`
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
func ProvideConfig() (*OperatorConfig, error) {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	if global == nil {
		cfg := OperatorConfig{}
		err := env.Parse(&cfg)
		if err != nil {
			return nil, err
		}
		
		cfg.IsAirGap = setAirGapStatus(&cfg)

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

		err = env.Parse(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse config")
		}

		if !inf.HasOpenshift() {
			cfg.RelatedImages = RelatedImages(cfg.OSRelatedImages)
		}

		cfg.IsAirGap = setAirGapStatus(cfg)

		global = cfg
	}

	return global, nil
}

func setAirGapStatus(cfg *OperatorConfig) (bool) {
	var rhmURL string

	rhmURL = utils.ProductionURL

	if cfg.URL != "" {
		rhmURL = cfg.URL
	}

	var ipLookUpFailed bool
	ip, err := net.LookupIP(rhmURL)
	if err != nil {
		ipLookUpFailed = checkError(err)
	}

	var dialTimeoutFailed bool
	u, _ := url.Parse(utils.ProductionURL)
	trimmedProdUrl := u.Host
	timeoutURL := fmt.Sprintf("%s:https", trimmedProdUrl)
	timeout := 1 * time.Second
	_, err = net.DialTimeout("tcp", timeoutURL, timeout)
	if err != nil {
		dialTimeoutFailed = checkError(err)
	}

	if ipLookUpFailed && dialTimeoutFailed {
		return true
	}

	log.Info("found IP for redhat marketplace", "ip", ip)
	return false
}

func checkError(err error) bool {
	if netError, ok := err.(net.Error); ok && netError.Timeout() {
		log.Info("DialTimeout exceeded timeout", "response", netError)
		return true
	}

	switch t := err.(type) {
	case *net.OpError:
		if t.Op == "dial" {
			log.Info("DialTimeout could not find host", "response", t)
			return true

		} else if t.Op == "read" {
			log.Info("DialTimeout connection refused", "response", t)
			return true
		}
	case *net.DNSError:
		if t.IsNotFound {
			log.Info("LookupIP could not find host", "response", t)
			return true
		}
	}

	return false
}

var GetConfig = ProvideConfig
