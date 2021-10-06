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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

	It("should replace resources", func() {
		factory := Factory{
			operatorConfig: &config.OperatorConfig{
				Config: config.EnvConfig{
					Resources: &config.Resources{
						Containers: map[string]v1.ResourceRequirements{
							"test": {
								Limits: v1.ResourceList{
									v1.ResourceCPU: resource.MustParse("100m"),
								},
								Requests: v1.ResourceList{
									v1.ResourceMemory: resource.MustParse("600Mi"),
								},
							},
						},
					},
				},
			},
		}

		container := v1.Container{
			Name: "test",
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("50m"),
					v1.ResourceMemory: resource.MustParse("1000Mi"),
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("25m"),
					v1.ResourceMemory: resource.MustParse("500Mi"),
				},
			},
		}

		Expect(factory.updateContainerResources(&container)).To(Succeed())

		r := resource.MustParse("100m")
		Expect(container.Resources.Limits.Cpu()).To(Equal(&r))
		r = resource.MustParse("1000Mi")
		Expect(container.Resources.Limits.Memory()).To(Equal(&r))
		r = resource.MustParse("25m")
		Expect(container.Resources.Requests.Cpu()).To(Equal(&r))
		r = resource.MustParse("600Mi")
		Expect(container.Resources.Requests.Memory()).To(Equal(&r))
	})
})
