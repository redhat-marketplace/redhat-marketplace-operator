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
	"context"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	osconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
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
			discoveryClient, _ := discovery.NewDiscoveryClientForConfig(cfg)
			cfg, err := ProvideInfrastructureAwareConfig(k8sClient, discoveryClient)

			Expect(err).To(Succeed())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Infrastructure).ToNot(BeNil())
			Expect(cfg.Infrastructure.Kubernetes).ToNot(BeNil())
			Expect(cfg.Infrastructure.KubernetesVersion()).NotTo(BeEmpty())
			Expect(cfg.Infrastructure.KubernetesPlatform()).NotTo(BeEmpty())
			Expect(cfg.Infrastructure.Openshift).To(BeNil())
		})
	})

	Context("with openshift information", func() {
		var clusterVersionObj *osconfigv1.ClusterVersion
		var ns *corev1.Namespace

		BeforeEach(func() {
			ns = &corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: "openshift-config",
				},
			}
			clusterVersionObj = &osconfigv1.ClusterVersion{
				ObjectMeta: v1.ObjectMeta{
					Name:      "version",
					Namespace: "openshift-config",
				},
				Spec: osconfigv1.ClusterVersionSpec{
					ClusterID: "foo",
					Channel:   "stable-4.6",
					DesiredUpdate: &osconfigv1.Update{
						Image:   "quay.io/openshift-release-dev/ocp-release@sha256:6ddbf56b7f9776c0498f23a54b65a06b3b846c1012200c5609c4bb716b6bdcdf",
						Version: "4.6.8",
					},
					Upstream: "https://api.openshift.com/api/upgrades_info/v1/graph",
				},
				Status: osconfigv1.ClusterVersionStatus{
					Desired: osconfigv1.Release{
						Version: "0.1.test",
					},
				},
			}

			err := k8sClient.Create(context.TODO(), ns)
			Expect(err).To(Succeed())
			err = k8sClient.Create(context.TODO(), clusterVersionObj)
			Expect(err).To(Succeed())
		})

		AfterEach(func() {
			err := k8sClient.Delete(context.TODO(), clusterVersionObj)
			Expect(err).To(Succeed())
		})

		It("should load infrastructure information with Openshift", func() {
			discoveryClient, _ := discovery.NewDiscoveryClientForConfig(cfg)
			i, err := ProvideInfrastructureAwareConfig(k8sClient, discoveryClient)

			Expect(err).To(Succeed())
			Expect(i.Infrastructure.Openshift).NotTo(BeNil())
			Expect(i.Infrastructure.OpenshiftVersion()).NotTo(BeEmpty())
			Expect(i.Infrastructure.OpenshiftVersion()).To(Equal("4.6.8"))
		})
	})
})
