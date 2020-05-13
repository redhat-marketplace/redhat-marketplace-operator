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
