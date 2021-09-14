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

		pair := model.SamplePair{
			Timestamp: model.Now(),
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
		Expect(results.IntervalStart).To(Equal(parsedDate))
		Expect(results.IntervalEnd).To(Equal(parsedDate.Add(time.Hour)))
		Expect(results.Value).To(Equal("1"))
	})
})
