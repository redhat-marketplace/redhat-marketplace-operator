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

package v1alpha1

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
)

const Version = "v1alpha1"

type MarketplaceReportSlice struct {
	ReportSliceID common.ReportSliceKey    `json:"report_slice_id"`
	Metadata      *SourceMetadata          `json:"metadata,omitempty"`
	Metrics       []*MarketplaceReportData `json:"metrics"`
}

type MarketplaceReportData struct {
	MetricID          string                          `json:"metric_id"`
	ReportPeriodStart common.Time                     `json:"report_period_start"`
	ReportPeriodEnd   common.Time                     `json:"report_period_end"`
	IntervalStart     common.Time                     `json:"interval_start"`
	IntervalEnd       common.Time                     `json:"interval_end"`
	MeterDomain       string                          `json:"domain"`
	MeterKind         string                          `json:"kind"`
	MeterVersion      string                          `json:"version,omitempty"`
	Label             string                          `json:"workload,omitempty"`
	ResourceNamespace string                          `json:"resource_namespace,omitempty"`
	ResourceName      string                          `json:"resource_name,omitempty"`
	Unit              string                          `json:"unit,omitempty"`
	AdditionalLabels  map[string]interface{}          `json:"additionalLabels"`
	Metrics           map[string]interface{}          `json:"rhmUsageMetrics"`
	MetricsExtended   []MarketplaceMetric             `json:"rhmUsageMetricsDetailed"`
	RecordSummary     *MarketplaceReportRecordSummary `json:"rhmUsageMetricsDetailedSummary,omitempty"`
}

type MarketplaceMetric struct {
	Label  string                 `json:"label"`
	Value  string                 `json:"value"`
	Labels map[string]interface{} `json:"labelSet,omitempty"`
}

type MarketplaceReportRecordSummary struct {
	TotalMetricCount int `json:"totalMetricCount"`
}
