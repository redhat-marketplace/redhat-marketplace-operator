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
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/common/model"

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
		metadata      *ReportMetadata
	)

	BeforeEach(func() {
		metricsReport = &MetricsReport{
			ReportSliceID: ReportSliceKey(sliceID),
		}
		key = MetricKey{
			MetricID:          "id",
			ReportPeriodStart: "start",
			ReportPeriodEnd:   "end",
			IntervalStart:     "istart",
			IntervalEnd:       "iend",
			MeterDomain:       "test",
			MeterKind:         "foo",
			MeterVersion:      "v1",
			ResourceName:      "foo",
			Namespace:         "bar",
			Label:             "awesome",
			Unit:              "unit",
		}
		metricBase = &MetricBase{
			Key: key,
		}
		metadata = NewReportMetadata(uuid.New(), ReportSourceMetadata{
			RhmClusterID:   "testCluster",
			RhmEnvironment: ReportSandboxEnv,
			RhmAccountID:   "testAccount",
		})

	})

	It("should serialize source metadata to json", func() {
		metadata := NewReportMetadata(uuid.New(), ReportSourceMetadata{
			RhmClusterID:   "testCluster",
			RhmEnvironment: ReportSandboxEnv,
			RhmAccountID:   "testAccount",
		})

		metadata.AddMetricsReport(metricsReport)

		data, err := json.Marshal(metadata)
		Expect(err).To(Succeed())

		u := ReportMetadata{}
		Expect(json.Unmarshal(data, &u)).To(Succeed())
		Expect(u.SourceMetadata.RhmEnvironment).To(Equal(ReportSandboxEnv))
	})

	It("should add metrics to a base", func() {

		metricsReport.AddMetadata(metadata.ToFlat())

		Expect(metricBase.AddMetrics("foo", 1, "bar", 2)).To(Succeed())
		Expect(metricBase.AddAdditionalLabels("extra", "g")).To(Succeed())
		Expect(metricsReport.AddMetrics(metricBase)).To(Succeed())
		Expect(len(metricsReport.Metrics)).To(Equal(1))

		id := func(element interface{}) string {
			return "0"
		}

		Expect(metricsReport).To(PointTo(MatchAllFields(Fields{
			"ReportSliceID": Equal(ReportSliceKey(sliceID)),
			"Metadata":      PointTo(Equal(*metadata.ToFlat())),
			"Metrics": MatchAllElements(id, Elements{
				"0": MatchAllKeys(Keys{
					"metric_id":           Equal("id"),
					"report_period_start": Equal("start"),
					"report_period_end":   Equal("end"),
					"interval_start":      Equal("istart"),
					"interval_end":        Equal("iend"),
					"domain":              Equal("test"),
					"version":             Equal("v1"),
					"kind":                Equal("foo"),
					"resource_name":       Equal("foo"),
					"namespace":           Equal("bar"),
					"workload":            Equal("awesome"),
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

		data, err := json.Marshal(metricsReport)
		Expect(err).To(Succeed())

		u := MetricsReport{}
		Expect(json.Unmarshal(data, &u)).To(Succeed())
		Expect(u.ReportSliceID).To(Equal(ReportSliceKey(sliceID)))
	})

	It("should create a new metric from values", func() {
		metric := NewMetric(
			model.SamplePair{
				Timestamp: model.Now(),
				Value:     model.SampleValue(0.01),
			},
		)
	})
})
