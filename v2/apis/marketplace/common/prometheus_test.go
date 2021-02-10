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

package common

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("MeterDefPrometheusLabels", func() {
	var promLabels *MeterDefPrometheusLabels

	BeforeEach(func() {

	})

	FIt("should convert to string", func() {
		vstr, err := convertToString("foo")
		Expect(err).To(Succeed())
		Expect(vstr).To(Equal("foo"))

		vstr, err = convertToString(JSONArray([]string{"c", "d"}))
		Expect(err).To(Succeed())
		Expect(vstr).To(Equal(`["c","d"]`))

		vstr, err = convertToString(&MetricPeriod{Duration: time.Hour})
		Expect(err).To(Succeed())
		Expect(vstr).To(Equal(`1h0m0s`))

		vstr, err = convertToString(MetricPeriod{Duration: time.Hour})
		Expect(err).To(Succeed())
		Expect(vstr).To(Equal(`1h0m0s`))

		vstr, err = convertToString(nil)
		Expect(err).To(HaveOccurred())
		Expect(vstr).To(Equal(``))
	})

	It("should turn into a label map", func() {
		promLabels = &MeterDefPrometheusLabels{
			MeterDefName:       "name",
			MeterDefNamespace:  "namespace",
			MeterGroup:         "group",
			MeterKind:          "kind",
			Metric:             "metric",
			DateLabelOverride:  "dateoverride",
			ValueLabelOverride: "valueoverride",
			MetricAggregation:  "sum",
			MetricGroupBy:      JSONArray([]string{"c", "d"}),
			MetricPeriod:       &MetricPeriod{Duration: time.Hour},
			MetricQuery:        "query",
			MetricWithout:      JSONArray([]string{"a", "b"}),
			UID:                "uid",
			WorkloadName:       "workloadname",
			WorkloadType:       "pod",
		}
		labelMap, err := promLabels.ToLabels()

		Expect(err).To(Succeed())
		Expect(labelMap).To(MatchAllKeys(Keys{
			"name":                 Equal("name"),
			"namespace":            Equal("namespace"),
			"metric_period":        Equal("1h0m0s"),
			"meter_group":          Equal("group"),
			"meter_kind":           Equal("kind"),
			"metric_label":         Equal("metric"),
			"date_label_override":  Equal("dateoverride"),
			"value_label_override": Equal("valueoverride"),
			"metric_aggregation":   Equal("sum"),
			"metric_group_by":      Equal(`["c","d"]`),
			"metric_without":       Equal(`["a","b"]`),
			"meter_definition_uid": Equal("uid"),
			"workload_type":        Equal("pod"),
			"workload_name":        Equal("workloadname"),
			"metric_query":         Equal("query"),
		}))

		newPromLabels := &MeterDefPrometheusLabels{}
		err = newPromLabels.FromLabels(labelMap)

		Expect(err).To(Succeed())

		Expect(*newPromLabels).To(MatchFields(IgnoreExtras, Fields{
			"MeterDefName":      Equal("name"),
			"MeterDefNamespace": Equal("namespace"),
			"MetricWithout":     Equal(JSONArray{"a", "b"}),
			"MetricPeriod":      PointTo(Equal(*newPromLabels.MetricPeriod)),
		}))

		promLabels.MetricGroupBy = nil
		promLabels.MetricWithout = JSONArray([]string{})

		labelMap, err = promLabels.ToLabels()

		Expect(err).To(Succeed())
		Expect(labelMap).To(MatchAllKeys(Keys{
			"name":                 Equal("name"),
			"namespace":            Equal("namespace"),
			"metric_period":        Equal("1h0m0s"),
			"meter_group":          Equal("group"),
			"meter_kind":           Equal("kind"),
			"metric_label":         Equal("metric"),
			"metric_aggregation":   Equal("sum"),
			"date_label_override":  Equal("dateoverride"),
			"value_label_override": Equal("valueoverride"),
			"metric_without":       Equal(`[]`),
			"meter_definition_uid": Equal("uid"),
			"workload_type":        Equal("pod"),
			"workload_name":        Equal("workloadname"),
			"metric_query":         Equal("query"),
		}))

		newPromLabels = &MeterDefPrometheusLabels{}
		err = newPromLabels.FromLabels(labelMap)

		Expect(err).To(Succeed())

		Expect(*newPromLabels).To(MatchFields(IgnoreExtras, Fields{
			"MeterDefName":      Equal("name"),
			"MeterDefNamespace": Equal("namespace"),
			"MetricWithout":     Equal(JSONArray{}),
		}))
	})
})
