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

package marketplace

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MeterbaseController", func() {
	Describe("check date functions", func() {
		var (
			ctrl *MeterBaseReconciler
		)

		BeforeEach(func() {
			ctrl = &MeterBaseReconciler{}
		})

		It("reports should calculate the correct dates to create", func() {
			endDate := time.Now().UTC()
			endDate = endDate.AddDate(0, 0, 0)
			minDate := endDate.AddDate(0, 0, 0)

			exp := ctrl.generateExpectedDates(endDate, time.UTC, -30, minDate)
			Expect(exp).To(HaveLen(1))

			minDate = endDate.AddDate(0, 0, -2)

			exp = ctrl.generateExpectedDates(endDate, time.UTC, -30, minDate)
			Expect(exp).To(HaveLen(3))
		})
	})
})
