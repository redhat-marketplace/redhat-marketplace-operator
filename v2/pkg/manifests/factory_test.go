// Copyright 2021 IBM Corp.
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

package manifests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/assets"
)

var files = []string{
	PrometheusOperatorDeploymentV45,
	PrometheusOperatorDeploymentV46,
	PrometheusOperatorServiceV45,
	PrometheusOperatorServiceV46,

	PrometheusAdditionalScrapeConfig,
	PrometheusHtpasswd,
	PrometheusRBACProxySecret,
	PrometheusDeploymentV45,
	PrometheusDeploymentV46,
	PrometheusProxySecret,
	PrometheusService,
	PrometheusDatasourcesSecret,
	PrometheusServingCertsCABundle,
	PrometheusKubeletServingCABundle,
	PrometheusServiceMonitor,
	PrometheusMeterDefinition,

	ReporterJob,
	ReporterUserWorkloadMonitoringJob,
	ReporterMeterDefinition,

	MetricStateDeployment,
	MetricStateServiceMonitorV45,
	MetricStateServiceMonitorV46,
	MetricStateService,
	MetricStateMeterDefinition,

	// ose-prometheus v4.6
	MetricStateRHMOperatorSecret,
	KubeStateMetricsService,
	KubeStateMetricsServiceMonitor,
	KubeletServiceMonitor,

	UserWorkloadMonitoringServiceMonitor,
	UserWorkloadMonitoringMeterDefinition,
}

var _ = Describe("FactoryTest", func() {
	It("should load files", func() {
		for _, file := range files {
			_, err := assets.ReadFile(file)
			Expect(err).ToNot(HaveOccurred(), file)
		}
	})
})
