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

package v2alpha1

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/test/v2alpha1buildertest"
)

var _ = Describe("Builder", func() {
	var (
		marketplaceReportSlice MarketplaceReportSlice
	)

	It("Unmarshals ReportFile1 successfully", func() {
		Expect(json.Unmarshal(v2alpha1buildertest.ReportFile1, &marketplaceReportSlice)).To(Succeed())
	})
	It("Unmarshals ReportFile1 unsuccessfully", func() {
		Expect(json.Unmarshal(v2alpha1buildertest.ReportFile1Bad, &marketplaceReportSlice)).To(Not(Succeed()))
	})
	It("Unmarshals ReportFile2 successfully", func() {
		Expect(json.Unmarshal(v2alpha1buildertest.ReportFile2, &marketplaceReportSlice)).To(Succeed())
	})
	It("Unmarshals ReportFile3 successfully", func() {
		Expect(json.Unmarshal(v2alpha1buildertest.ReportFile3, &marketplaceReportSlice)).To(Succeed())
	})
})
