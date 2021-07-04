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
	"context"
	"net"

	"os"

	"github.com/foxcpp/go-mockdns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	osconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
)

var _ = Describe("Config", func() {
	var discoveryClient *discovery.DiscoveryClient
	var srv *mockdns.Server
	mockURL := "mock.marketplace.com."

	BeforeEach(func() {
		srv, _ = mockdns.NewServer(map[string]mockdns.Zone{
			mockURL: {},
		}, false)
	
		os.Setenv("MARKETPLACE_URL","mock.marketplace.com")
		discoveryClient, _ = discovery.NewDiscoveryClientForConfig(cfg)
		reset()
	})

	AfterEach(func(){
		srv.Close()
		mockdns.UnpatchNet(net.DefaultResolver)
		os.Unsetenv("MARKETPLACE_URL")
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

		BeforeEach(func(){
			srv.Resolver().Zones[mockURL] = mockdns.Zone{A: []string{"1.2.3.4"}}
			srv.PatchNet(net.DefaultResolver)
		})

		It("should load infrastructure information", func() {
			cfg, err := ProvideInfrastructureAwareConfig(k8sClient, discoveryClient)

			Expect(err).To(Succeed())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Infrastructure.KubernetesVersion()).NotTo(BeEmpty())
			Expect(cfg.Infrastructure.KubernetesPlatform()).NotTo(BeEmpty())
			Expect(cfg.Infrastructure.HasOpenshift()).To(BeFalse())
			Expect(cfg.IsAirGap).To(BeFalse(),"is airgap should be false")
		})
	})

	Context("With AirGap environment information",func() {
		BeforeEach(func(){
			srv.Resolver().Zones[mockURL] = mockdns.Zone{A: []string{}}
			srv.PatchNet(net.DefaultResolver)
		})

		It("Should set the IsAirGap var on OperatorConfig", func(){
			cfg, err := ProvideInfrastructureAwareConfig(k8sClient, discoveryClient)

			Expect(err).To(Succeed())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.IsAirGap).To(BeTrue())
		})
	})

	Context("with openshift information", func() {
		var clusterVersionObj *osconfigv1.ClusterVersion

		BeforeEach(func() {
			clusterVersionString := `apiVersion: config.openshift.io/v1
kind: ClusterVersion
metadata:
  creationTimestamp: "2020-11-18T12:58:44Z"
  generation: 4
  labels:
    razee/watch-resource: detail
  name: version
spec:
  channel: fast-4.6
  clusterID: 969afac0-d784-4ad1-a3fd-aabb68e9742f
  upstream: https://api.openshift.com/api/upgrades_info/v1/graph
status:
  availableUpdates:
  - channels:
    - candidate-4.6
    - candidate-4.7
    - eus-4.6
    - fast-4.6
    - stable-4.6
    image: quay.io/openshift-release-dev/ocp-release@sha256:5c3618ab914eb66267b7c552a9b51c3018c3a8f8acf08ce1ff7ae4bfdd3a82bd
    url: https://access.redhat.com/errata/RHSA-2021:0037
    version: 4.6.12
  - channels:
    - candidate-4.6
    - candidate-4.7
    - eus-4.6
    - fast-4.6
    - stable-4.6
    image: quay.io/openshift-release-dev/ocp-release@sha256:6ddbf56b7f9776c0498f23a54b65a06b3b846c1012200c5609c4bb716b6bdcdf
    url: https://access.redhat.com/errata/RHSA-2020:5259
    version: 4.6.8
  conditions:
  - lastTransitionTime: "2020-11-18T13:28:20Z"
    message: Done applying 4.6.4
    status: "True"
    type: Available
  - lastTransitionTime: "2020-11-18T13:28:20Z"
    status: "False"
    type: Failing
  - lastTransitionTime: "2020-11-18T13:28:20Z"
    message: Cluster version is 4.6.4
    status: "False"
    type: Progressing
  - lastTransitionTime: "2021-01-19T16:32:08Z"
    status: "True"
    type: RetrievedUpdates
  desired:
    channels:
    - candidate-4.6
    - eus-4.6
    - fast-4.6
    - stable-4.6
    image: quay.io/openshift-release-dev/ocp-release@sha256:6681fc3f83dda0856b43cecd25f2d226c3f90e8a42c7144dbc499f6ee0a086fc
    url: https://access.redhat.com/errata/RHBA-2020:4987
    version: 4.6.4
  history:
  - completionTime: "2020-11-18T13:28:20Z"
    image: quay.io/openshift-release-dev/ocp-release@sha256:6681fc3f83dda0856b43cecd25f2d226c3f90e8a42c7144dbc499f6ee0a086fc
    startedTime: "2020-11-18T12:58:46Z"
    state: Completed
    verified: false
    version: 4.6.4
  observedGeneration: 4
  versionHash: 27XnFTHcOiw=`

			clusterVersionObj = &osconfigv1.ClusterVersion{}
			err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(clusterVersionString)), 100).Decode(clusterVersionObj)
			Expect(err).To(Succeed())

			err = k8sClient.Create(context.TODO(), clusterVersionObj)
			Expect(err).To(Succeed())

			err = k8sClient.Status().Update(context.TODO(), clusterVersionObj)
			Expect(err).To(Succeed())
		})

		AfterEach(func() {
			err := k8sClient.Delete(context.TODO(), clusterVersionObj)
			Expect(err).To(Succeed())
		})

		It("should load infrastructure information with Openshift", func() {
			i, err := ProvideInfrastructureAwareConfig(k8sClient, discoveryClient)

			Expect(err).To(Succeed())
			Expect(i.Infrastructure.OpenshiftVersion()).NotTo(BeEmpty())
			Expect(i.Infrastructure.OpenshiftVersion()).To(Equal("4.6.4"))
		})
	})
})
