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

package v2alpha1

const Version = "v2alpha1"

type MarketplaceReportSlice struct {
	/*
		This is required only because RH Insights forces the metadata field.
		It will exist only in the case where data is sent through the RH pipeline to the COS bucket directly.
		It will not be inspected or processed.
	*/
	Metadata *SourceMetadata `json:"metadata,omitempty"`
	// An array of metrics data objects.
	Metrics []*MarketplaceReportData `json:"data"`
}

type MarketplaceReportData struct {
	/*
		A GUID uniquely identifying this event being sent.
		A second event with the same eventId will be processed as an amendment.
		Two events in the same uploaded archive MUST NOT have the same event id. This may be across all the files in the archive.
	*/
	EventID string `json:"eventId"`
	// Milliseconds from epoch UTC representing the start of the window for the usage data being reported
	IntervalStart int64 `json:"start"`
	// Milliseconds from epoch UTC representing the end of the window for the usage data being reported
	IntervalEnd int64 `json:"end"`
	// The Id of the Red Hat Marketplace account that usage is being reported for.
	AccountID string `json:"accountId" mapstructure:"accountId"`
	/*
		An object allowing for additional attributes related to the usage data.
		This object will be merged with measuredUsage.additionalAttributes for processing.
		See measuredUsage.additionalAttributes for details and the Additional Attributes section below.
	*/
	AdditionalAttributes map[string]interface{} `json:"additionalAttributes"`
	/*
		A group name indicates a chargeback/tag group for the usage coming from containerized environments.
		This will be used to identify a namespace, the labels on the namespace and apply matching RHM tags to the usage.
	*/
	GroupName string `json:"groupName"`
	// An array of usage objects.
	MeasuredUsage []MeasuredUsage `json:"measuredUsage"`
}

type MeasuredUsage struct {
	/*
		The ID of the metric being reported.
		If multiple objects inside measuredUsage have the same metricId for the same product the complete event will be rejected and NOT processed.
	*/
	MetricID string `json:"metricId"`
	// The recorded usage value for the time window.
	Value float64 `json:"value"`
	/*
		The additionalAttributes associated with the metric will be merged with the additionalAttributes for the event overall.
		Priority will be give to metric level additionalAttributes
	*/
	AdditionalAttributes map[string]interface{} `json:"additionalAttributes"`
}
