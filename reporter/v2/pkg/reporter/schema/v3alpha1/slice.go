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

package v3alpha1

const Version = "v3alpha1"

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

	// --- Usage event properties ---

	//A value uniquely identifying the event. A second event with the same eventId will be processed as an amendment. Two events in the same archive MUST NOT have the same eventId. Recommended to be a GUID.
	EventID string `json:"eventId"`

	// Milliseconds from epoch UTC representing the start of the window for the usage data being reported
	IntervalStart int64 `json:"start,omitempty"`

	// Milliseconds from epoch UTC representing the end of the window for the usage data being reported
	IntervalEnd int64 `json:"end,omitempty"`

	// The id of the IBM Software Central or Red Hat Marketplace account that usage is being reported for.
	AccountID string `json:"accountId,omitempty" mapstructure:"accountId"`

	// The id of the IBM Software Central or Red Hat Marketplace subscription for the product that usage is being reported for.
	SubscriptionId string `json:"subscriptionId,omitempty" mapstructure:"subscriptionId"`

	// An array of usage objects.
	MeasuredUsage []MeasuredUsage `json:"measuredUsage"`

	// --- Additional Properties ---

	Source                         string `json:"source,omitempty"`
	SourceSaas                     string `json:"sourceSaas,omitempty"`
	AccountIdSaas                  string `json:"accountIdSaas,omitempty"`
	SubscriptionIdSaas             string `json:"subscriptionIdSaas,omitempty"`
	ProductType                    string `json:"productType,omitempty"`
	LicensePartNumber              string `json:"licensePartNumber,omitempty"`
	ProductId                      string `json:"productId,omitempty"`
	SapEntitlementLine             string `json:"sapEntitlementLine,omitempty"`
	ProductName                    string `json:"productName,omitempty"`
	ParentProductId                string `json:"parentProductId,omitempty"`
	ParentProductName              string `json:"parentProductName,omitempty"`
	ParentProductMetricId          string `json:"parentProductMetricId,omitempty"`
	TopLevelProductId              string `json:"topLevelProductId,omitempty"`
	TopLevelProductName            string `json:"topLevelProductName,omitempty"`
	TopLevelProductMetricId        string `json:"topLevelProductMetricId,omitempty"`
	DswOfferAccountingSystemCode   string `json:"dswOfferAccountingSystemCode,omitempty"`
	DswSubscriptionAgreementNumber string `json:"dswSubscriptionAgreementNumber,omitempty"`
	SsmSubscriptionId              string `json:"ssmSubscriptionId,omitempty"`
	ICN                            string `json:"ICN,omitempty"`
	Group                          string `json:"group,omitempty"`
	GroupName                      string `json:"groupName,omitempty"`
	Kind                           string `json:"kind,omitempty"`
}

type MeasuredUsage struct {

	// The ID of the metric being reported. If multiple objects inside measuredUsage have the same metricId for the same product the complete event will be rejected and NOT processed.
	MetricID string `json:"metricId"`

	// The recorded usage value for the time window.
	Value float64 `json:"value"`

	// --- Additional Properties ---

	MetricType             string            `json:"metricType,omitempty"`
	MetricAggregationType  string            `json:"metricAggregationType,omitempty"`
	MeasuredMetricId       string            `json:"measuredMetricId,omitempty"`
	ProductConversionRatio string            `json:"productConversionRatio,omitempty"`
	MeasuredValue          string            `json:"measuredValue,omitempty"`
	ClusterId              string            `json:"clusterId,omitempty"`
	Hostname               string            `json:"hostname,omitempty"`
	NamespacesLabels       []NamespaceLabels `json:"namespace,omitempty"`
	Pod                    string            `json:"pod,omitempty"`
	PlatformId             string            `json:"platformId,omitempty"`
	Meter_def_namespace    string            `json:"meter_def_namespace,omitempty"`
	Crn                    string            `json:"crn,omitempty"`
	IsViewable             string            `json:"isViewable,omitempty"`
	CalculateSummary       string            `json:"calculateSummary,omitempty"`
}

type NamespaceLabels struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}
