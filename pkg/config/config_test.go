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
)

var _ = Describe("Config", func() {
	Context("with defaults", func() {
		It("should set defaults", func() {

			cfg, err := ProvideConfig()
			Expect(err).To(Succeed())
			Expect(cfg).ToNot(BeNil())

			Expect(cfg.RelatedImages.Reporter).To(Equal("reporter:latest"))
			Expect(cfg.IBMCatalog).To(BeTrue())
		})
	})

	Context("with features flag", func() {
		BeforeEach(func() {
			os.Setenv("FEATURE_IBMCATALOG", "false")
			os.Setenv("RELATED_IMAGE_METRIC_STATE", "foo")
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
})
