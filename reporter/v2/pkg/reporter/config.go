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

package reporter

import (
	"github.com/google/wire"
	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

type Name types.NamespacedName
type PrometheusService *corev1.Service
type ReportOutputDir string

// Top level config
type Config struct {
	OutputDirectory      string
	MetricsPerFile       *int
	MaxRoutines          *int
	Retry                *int
	CaFile               string
	TokenFile            string
	DataServiceTokenFile string
	DataServiceCertFile  string
	DeployedNamespace    string
	Local                bool
	Upload               bool
	IsDisconnected       bool
	PrometheusService    string
	PrometheusNamespace  string
	PrometheusPort       string
	uploaders.UploaderTargets
	ReporterSchema string
	CipherSuites   []uint16
	MinVersion     uint16

	K8sRestConfig *rest.Config
}

const (
	defaultMetricsPerFile = 500
	defaultMaxRoutines    = 50
)

func (c *Config) SetDefaults() error {
	if c.MetricsPerFile == nil {
		c.MetricsPerFile = ptr.Int(defaultMetricsPerFile)
	}

	if c.MaxRoutines == nil {
		c.MaxRoutines = ptr.Int(defaultMaxRoutines)
	}

	if c.Retry == nil {
		c.Retry = ptr.Int(5)
	}

	if c.UploaderTargets == nil {
		c.UploaderTargets = uploaders.UploaderTargets{&dataservice.DataService{}}
	}

	if c.K8sRestConfig == nil {
		var err error
		c.K8sRestConfig, err = kconfig.GetConfig()
		if err != nil {
			logger.Error(err, "failed to get config")
			return err
		}
	}

	return nil
}

var ReporterSet = wire.NewSet(
	NewMarketplaceReporter,
	uploaders.NewRedHatInsightsUploader,
)
