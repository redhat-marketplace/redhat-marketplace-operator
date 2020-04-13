package reporter

import (
	"testing"

	"github.com/google/uuid"
)

func TestMetricBuilder(t *testing.T) {
	metricsReport := &MetricsReport{
		ReportSliceID: ReportSliceKey(uuid.New()),
	}
	metricBase := &MetricBase{
		ReportPeriodStart: "start",
		ReportPeriodEnd:   "end",
		IntervalStart:     "istart",
		IntervalEnd:       "iend",
		MeterDomain:       "test",
	}

	err := metricBase.AddMetrics("foo", 1, "bar", 2)
	if err != nil {
		t.Fatalf("expected no error %v", err)
	}

	err = metricsReport.AddMetrics(metricBase)
	if err != nil {
		t.Fatalf("expected no error %v", err)
	}

	size := len(metricsReport.Metrics)
	if size != 1 {
		t.Fatalf("len(metrics) = %d; want 1", size)
	}

	fooValue, fooPresent := metricsReport.Metrics[0]["foo"]

	if !fooPresent {
		t.Fatalf("expected foo to be a metric")
	}

	if fooInt, ok := fooValue.(int); ok && fooInt != 1 {
		t.Fatalf("fooValue = %d; want 1", fooInt)
	}

	barValue, barPresent := metricsReport.Metrics[0]["bar"]

	if !barPresent {
		t.Fatalf("expected bar to be a metric")
	}

	if barInt, ok := barValue.(int); ok && barInt != 2 {
		t.Fatalf("barValue = %d; want 2", barInt)
	}
}
