package dataservice

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
)

var _ = Describe("metadata", func() {
	var (
		testMap = map[string]string{
			"reportName":      "foo",
			"reportNamespace": "foo-ns",
			"irrelevant":      "hi",
		}
	)

	It("should parse from map[string]string", func() {
		m := &MeterReportMetadata{}
		Expect(m.From(testMap)).To(Succeed())
		Expect(m.IsEmpty()).To(BeFalse())
		Expect(*m).To(gstruct.MatchAllFields(gstruct.Fields{
			"ReportName":      Equal("foo"),
			"ReportNamespace": Equal("foo-ns"),
		}))
		Expect(m.ReportNamespace).To(Equal("foo-ns"))
		m2, err := m.Map()
		Expect(err).To(Succeed())
		Expect(m2).To(gstruct.MatchAllKeys(gstruct.Keys{
			"reportName":      Equal("foo"),
			"reportNamespace": Equal("foo-ns"),
		}))
		m3 := &MeterReportMetadata{}
		Expect(m3.From(map[string]string{})).To(Succeed())
		Expect(m3.IsEmpty()).To(BeTrue())
	})
})
