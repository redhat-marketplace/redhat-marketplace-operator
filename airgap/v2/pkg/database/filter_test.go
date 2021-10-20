package database

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("filter", func() {
	It("should parse to a string", func() {
		Skip("wip")
		f, err := ParseFilter("deletedAt > 0")
		Expect(err).To(Succeed())
		Expect(f).To(Equal("deletedAt > 0"))

		f, err = ParseFilter(`source=="foo"`)
		Expect(err).To(Succeed())
		Expect(f).To(Equal("source == \"foo\""))

		f, err = ParseFilter(`(source == "foo") && deletedAt > 0`)
		Expect(err).To(Succeed())
		Expect(f).To(Equal("source == \"foo\" AND deletedAt > 0"))
	})
})
