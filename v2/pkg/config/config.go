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
	"fmt"
	"net"
	"net/url"
	"reflect"
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
	MeterdefinitionCatalog
	Config              EnvConfig `env:"CONFIG"`
	IsDisconnected      bool      `env:"IS_DISCONNECTED" envDefault:"false"`
	DataServiceReplicas int       `env:"DATA_SERVICE_REPLICAS" envDefault:"3"`
	minVersion          string
	cipherSuites        []string
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
	Reporter           string `env:"RELATED_IMAGE_REPORTER" envDefault:"reporter:latest"`
	KubeRbacProxy      string `env:"RELATED_IMAGE_KUBE_RBAC_PROXY" envDefault:"registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.14"`
	MetricState        string `env:"RELATED_IMAGE_METRIC_STATE" envDefault:"metric-state:latest"`
	AuthChecker        string `env:"RELATED_IMAGE_AUTHCHECK" envDefault:"authcheck:latest"`
	DQLite             string `env:"RELATED_IMAGE_DQLITE" envDefault:"dqlite:latest"`
	MeterDefFileServer string `env:"RELATED_IMAGE_METERDEF_FILE_SERVER" envDefault:"quay.io/rh-marketplace/rhm-meterdefinition-file-server:v1"`
}

// OSRelatedImages stores open source related images for the operator
type OSRelatedImages struct {
	Reporter           string `env:"RELATED_IMAGE_REPORTER" envDefault:"reporter:latest"`
	KubeRbacProxy      string `env:"OS_IMAGE_KUBE_RBAC_PROXY" envDefault:"quay.io/brancz/kube-rbac-proxy:latest"`
	MetricState        string `env:"RELATED_IMAGE_METRIC_STATE" envDefault:"metric-state:latest"`
	AuthChecker        string `env:"RELATED_IMAGE_AUTHCHECK" envDefault:"authcheck:latest"`
	DQLite             string `env:"RELATED_IMAGE_DQLITE" envDefault:"dqlite:latest"`
	MeterDefFileServer string `env:"RELATED_IMAGE_METERDEF_FILE_SERVER" envDefault:"quay.io/rh-marketplace/rhm-meterdefinition-file-server:v1"`
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

type MeterdefinitionCatalog struct {
	ImageStreamValues
	FileServerValues
}

type FileServerValues struct {
	FileServerURL string `env:"CATALOG_URL" envDefault:""`
}

type ImageStreamValues struct {
	ImageStreamID  string `env:"IMAGE_STREAM_ID" envDefault:"rhm-meterdefinition-file-server:v1"`
	ImageStreamTag string `env:"IMAGE_STREAM_TAG" envDefault:"v1"`
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

		cfg.IsDisconnected = setDisconnectedStatus(&cfg)

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

		if !inf.HasOpenshift() {
			cfg.RelatedImages = RelatedImages(cfg.OSRelatedImages)
		}

		global = cfg
	}

	return global, nil
}

func setDisconnectedStatus(cfg *OperatorConfig) bool {
	var rhmURL string

	rhmURL = utils.ProductionURL

	if cfg.URL != "" {
		rhmURL = cfg.URL
	}

	var ipLookUpFailed bool
	_, err := net.LookupIP(rhmURL)
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
		log.Info("ip lookup and timeout failed")
		return true
	}

	log.Info("found IP for redhat marketplace")
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
