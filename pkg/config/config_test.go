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
