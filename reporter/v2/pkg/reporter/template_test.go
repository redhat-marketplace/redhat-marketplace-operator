package reporter

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
)

var _ = Describe("Template", func() {
	var promLabels *common.MeterDefPrometheusLabels

	It("should evaluate templates", func() {
		values := &ReportLabels{
			Labels: map[string]interface{}{
				"foo_bar_label":     "label",
				"a_date_label":      "2020-02-11",
				"a_timestamp_label": "2020-02-11",
			},
		}
		promLabels = &common.MeterDefPrometheusLabels{
			MeterDefName:       "{{ foo_bar_label }}",
			MeterDefNamespace:  "namespace",
			MeterGroup:         "{{ .Labels.foo_bar_label }}.group",
			MeterKind:          "{{ .Labels.foo_bar_label }}.kind",
			Metric:             "{{ .Labels.foo_bar_label }}.metric",
			DateLabelOverride:  "{{ .Labels.foo_bar_label }}.date",
			ValueLabelOverride: "{{ .Labels.foo_bar_label }}.valueoverride",
			MeterDescription:   "{{ .Labels.foo_bar_label }}.description",
			MetricAggregation:  "sum",
			MetricGroupBy:      common.JSONArray([]string{"c", "d"}),
			MetricPeriod:       &common.MetricPeriod{Duration: time.Hour},
			MetricQuery:        "query",
			MetricWithout:      common.JSONArray([]string{"a", "b"}),
			UID:                "uid",
			WorkloadName:       "workloadname",
			WorkloadType:       "pod",
		}

		templ, err := NewTemplate(promLabels)
		Expect(err).To(Succeed())
		Expect(len(templ.templFieldMap)).ToNot(BeZero())
		templ.Execute(promLabels, values)

		Expect(promLabels).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"MeterDefName":       Equal("{{ foo_bar_label }}"), //shouldn't change, this isn't a template field
			"MeterGroup":         Equal("label.group"),
			"MeterKind":          Equal("label.kind"),
			"Metric":             Equal("label.metric"),
			"DateLabelOverride":  Equal("label.date"),
			"ValueLabelOverride": Equal("label.valueoverride"),
			"WorkloadType":       Equal("pod"),
		})))
	})
})
