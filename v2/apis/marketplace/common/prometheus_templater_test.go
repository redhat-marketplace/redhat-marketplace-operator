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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/prometheus/common/model"
)

var _ = Describe("Template", func() {
	var promLabels *MeterDefPrometheusLabels

	It("should evaluate templates", func() {
		values := &ReportLabels{
			Label: map[string]interface{}{
				"foo_bar_label":     "label",
				"a_date_label":      "2020-02-11",
				"a_timestamp_label": "2020-02-11",
				"value":             "1",
			},
		}
		promLabels = &MeterDefPrometheusLabels{
			MeterDefName:       "{{ foo_bar_label }}",
			MeterDefNamespace:  "namespace",
			MeterGroup:         "{{ .Label.foo_bar_label }}.group",
			MeterKind:          "{{ .Label.foo_bar_label }}.kind",
			Metric:             "{{ .Label.foo_bar_label }}.metric",
			DateLabelOverride:  "{{ .Label.a_date_label }}",
			ValueLabelOverride: "{{ .Label.value  }}",
			DisplayName:        "{{ .Label.foo_bar_label }}.name",
			MeterDescription:   "{{ .Label.foo_bar_label }}.description",
			MetricAggregation:  "sum",
			MetricGroupBy:      JSONArray([]string{"c", "d"}),
			MetricPeriod:       &MetricPeriod{Duration: time.Hour},
			MetricQuery:        "query",
			MetricWithout:      JSONArray([]string{"a", "b"}),
			UID:                "uid",
			WorkloadName:       "workloadname",
			WorkloadType:       "Pod",
		}

		setT, _ := time.Parse(time.RFC3339, "2020-02-11T20:04:05Z-05:00")
		now := model.TimeFromUnixNano(setT.UnixNano())

		pair := model.SamplePair{
			Timestamp: now,
			Value:     model.SampleValue(1.0),
		}

		results, err := promLabels.PrintTemplate(values, pair)
		Expect(err).To(Succeed())

		Expect(results.MeterDefPrometheusLabels).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"MeterDefName": Equal("{{ foo_bar_label }}"), //shouldn't change, this isn't a template field
			"MeterGroup":   Equal("label.group"),
			"MeterKind":    Equal("label.kind"),
			"Metric":       Equal("label.metric"),
			"DisplayName":  Equal("label.name"),
			"WorkloadType": Equal(WorkloadTypePod),
		})))

		parsedDate, _ := time.Parse(justDateFormat, "2020-02-11")
		year, month, day := parsedDate.UTC().Date()
		hour, minute, second := now.Time().UTC().Clock()

		expectedDate := time.Date(year, month, day, hour, minute, second, 0, time.UTC)

		Expect(results.IntervalStart).To(Equal(expectedDate))
		Expect(results.IntervalEnd).To(Equal(expectedDate.Add(time.Hour)))
		Expect(results.Value).To(Equal("1"))

		pair2 := model.SamplePair{
			Timestamp: now.Add(time.Hour),
			Value:     model.SampleValue(1.0),
		}
		results, err = promLabels.PrintTemplate(values, pair2)
		Expect(err).To(Succeed())

		Expect(results.IntervalStart).To(Equal(expectedDate.Add(time.Hour)))
		Expect(results.IntervalEnd).To(Equal(expectedDate.Add(2 * time.Hour)))
		Expect(results.Value).To(Equal("1"))
	})
})
