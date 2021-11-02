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
