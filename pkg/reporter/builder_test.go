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
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Builder", func() {
	var (
		metricsReport *MetricsReport
		key           MetricKey
		metricBase    *MetricBase
		sliceID       = uuid.New()
	)

	BeforeEach(func() {
		metricsReport = &MetricsReport{
			ReportSliceID: ReportSliceKey(sliceID),
		}
		key = MetricKey{
			ReportPeriodStart: "start",
			ReportPeriodEnd:   "end",
			IntervalStart:     "istart",
			IntervalEnd:       "iend",
			MeterDomain:       "test",
			MeterKind:         "foo",
			MeterVersion:      "v1",
		}
		metricBase = &MetricBase{
			Key: key,
		}
	})

	It("should add metrics to a base", func() {
		Expect(metricBase.AddMetrics("foo", 1, "bar", 2)).To(Succeed())
		Expect(metricBase.AddAdditionalLabels("extra", "g")).To(Succeed())
		Expect(metricsReport.AddMetrics(metricBase)).To(Succeed())
		Expect(len(metricsReport.Metrics)).To(Equal(1))

		id := func(element interface{}) string {
			return "0"
		}

		Expect(metricsReport).To(PointTo(MatchAllFields(Fields{
			"ReportSliceID": Equal(ReportSliceKey(sliceID)),
			"Metrics": MatchAllElements(id, Elements{
				"0": MatchAllKeys(Keys{
					"report_period_start": Equal("start"),
					"report_period_end":   Equal("end"),
					"interval_start":      Equal("istart"),
					"interval_end":        Equal("iend"),
					"domain":              Equal("test"),
					"version":             Equal("v1"),
					"kind":                Equal("foo"),
					"additionalLabels": MatchAllKeys(Keys{
						"extra": Equal("g"),
					}),
					"rhmUsageMetrics": MatchAllKeys(Keys{
						"foo": Equal(1),
						"bar": Equal(2),
					}),
				}),
			}),
		})))
	})
})
