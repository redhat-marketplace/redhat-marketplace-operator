package reporter

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
)

var _ = Describe("discover", func() {
	var data model.Matrix
	BeforeEach(func() {
		promLabels := &common.MeterDefPrometheusLabels{
			MeterDefName:       "name",
			MeterDefNamespace:  "namespace",
			MeterGroup:         "group",
			MeterKind:          "kind",
			Metric:             "metric",
			DateLabelOverride:  "dateoverride",
			ValueLabelOverride: "valueoverride",
			MetricAggregation:  "sum",
			MetricGroupBy:      common.JSONArray([]string{"c", "d"}),
			MetricPeriod:       &common.MetricPeriod{Duration: time.Hour},
			MetricQuery:        "query",
			MetricWithout:      common.JSONArray([]string{"a", "b"}),
			UID:                "uid",
			WorkloadName:       "workloadname",
			WorkloadType:       "Pod",
		}
		labelMap, err := promLabels.ToLabels()
		Expect(err).To(Succeed())
		metricMap := model.Metric{}

		for k, v := range labelMap {
			metricMap[model.LabelName(k)] = model.LabelValue(v)
		}

		Expect(metricMap[model.LabelName("metric_period")]).To(Equal(model.LabelValue("1h0m0s")))

		data = model.Matrix{
			&model.SampleStream{
				Metric: metricMap,
				Values: []model.SamplePair{
					{
						Timestamp: model.TimeFromUnix(1612828800),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612832400),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612836000),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612839600),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612843200),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612846800),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612850400),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612854000),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612857600),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612861200),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612864800),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612868400),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612872000),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612875600),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612879200),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612882800),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612886400),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612890000),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612893600),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612897200),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612900800),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612904400),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612908000),
						Value:     model.SampleValue(1),
					},
					{
						Timestamp: model.TimeFromUnix(1612911600),
						Value:     model.SampleValue(1),
					},
				},
			},
		}
	})

	It("should parse the date correctly", func() {
		results, errs := getQueries(data)
		Expect(len(errs)).To(Equal(0))
		Expect(len(results)).To(Equal(1))

		query := results[0].query

		endTime := model.TimeFromUnix(1612911600).Add(time.Hour).Time().UTC()

		Expect(query.End).To(Equal(endTime), "times should match")
	})
})
