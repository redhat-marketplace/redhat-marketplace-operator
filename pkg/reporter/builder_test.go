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

package reporter

import (
	"github.com/google/uuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Builder", func() {
	var (
		metricsReport *MetricsReport
		key           MetricKey
		metricBase    *MetricBase
	)

	BeforeEach(func() {
		metricsReport = &MetricsReport{
			ReportSliceID: ReportSliceKey(uuid.New()),
		}
		key = MetricKey{
			ReportPeriodStart: "start",
			ReportPeriodEnd:   "end",
			IntervalStart:     "istart",
			IntervalEnd:       "iend",
			MeterDomain:       "test",
		}
		metricBase = &MetricBase{
			Key: key,
		}
	})

	It("should add metrics to a base", func() {
		Expect(metricBase.AddMetrics("foo", 1, "bar", 2)).To(Succeed())
		Expect(metricsReport.AddMetrics(metricBase)).To(Succeed())
		Expect(len(metricsReport.Metrics)).To(Equal(1))

		fooValue, fooPresent := metricsReport.Metrics[0]["foo"]
		Expect(fooPresent).To(BeTrue())

		fooInt, ok := fooValue.(int)

		Expect(ok).To(BeTrue())
		Expect(fooInt).To(Equal(1))

		barValue, barPresent := metricsReport.Metrics[0]["bar"]

		Expect(barPresent).To(BeTrue())

		barInt, ok := barValue.(int)

		Expect(ok).To(BeTrue())
		Expect(barInt).To(Equal(2))
	})
})
