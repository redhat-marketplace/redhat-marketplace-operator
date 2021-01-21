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
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Config", func() {
	BeforeEach(func() {
		reset()
	})
	Context("with defaults", func() {
		It("should set defaults", func() {

			cfg, err := ProvideConfig()
			Expect(err).To(Succeed())
			Expect(cfg).ToNot(BeNil())

			Expect(cfg.RelatedImages.Reporter).To(Equal("reporter:latest"))
			Expect(cfg.Features.IBMCatalog).To(BeTrue())
		})
	})

	Context("with features flag", func() {
		BeforeEach(func() {
			os.Setenv("FEATURE_IBMCATALOG", "false")
			os.Setenv("RELATED_IMAGE_METRIC_STATE", "foo")
			reset()
		})

		AfterEach(func() {
			os.Unsetenv("FEATURE_IBMCATALOG")
			os.Unsetenv("RELATED_IMAGE_METRIC_STATE")
		})

		It("should have a feature flag", func() {
			cfg, err := ProvideConfig()

			Expect(err).To(Succeed())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Features.IBMCatalog).To(BeFalse())
			Expect(cfg.RelatedImages.MetricState).To(Equal("foo"))
		})
	})

	Context("with infrastructure information", func() {
		It("should load infrastructure information", func() {
			restConfig, _ := config.GetConfig()
			discoveryClient, _ := discovery.NewDiscoveryClientForConfig(restConfig)
			client := fake.NewFakeClient()
			cfg, err := ProvideInfrastructureAwareConfig(client, discoveryClient)

			Expect(err).To(Succeed())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Infrastructure).ToNot(BeNil())
			Expect(cfg.Infrastructure.Kubernetes).ToNot(BeNil())
			Expect(cfg.Infrastructure.KubernetesVersion()).NotTo(BeEmpty())
			Expect(cfg.Infrastructure.KubernetesPlatform()).NotTo(BeEmpty())
			Expect(cfg.Infrastructure.Openshift).To(BeNil())
		})
		It("should load infrastructure information with Openshift", func() {
			restConfig, _ := config.GetConfig()
			discoveryClient, _ := discovery.NewDiscoveryClientForConfig(restConfig)
			clusterVersionObj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "config.openshift.io/v1",
					"kind":       "ClusterVersion",
					"metadata": map[string]interface{}{
						"name":  "version",
						"value": "test",
					},
					"spec": "console",
					"status": map[string]interface{}{
						"desired": map[string]interface{}{
							"version": "0.1.test",
						},
					},
				},
			}
			client := fake.NewFakeClient(clusterVersionObj)
			cfg, err := ProvideInfrastructureAwareConfig(client, discoveryClient)

			Expect(err).To(Succeed())
			Expect(cfg.Infrastructure.OpenshiftVersion()).NotTo(BeEmpty())
		})
	})
})
