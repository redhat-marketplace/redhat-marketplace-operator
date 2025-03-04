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

import "encoding/json"

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
	EventID string `json:"eventId" mapstructure:"-"`

	// Milliseconds from epoch UTC representing the start of the window for the usage data being reported
	IntervalStart int64 `json:"start,omitempty" mapstructure:"-"`

	// Milliseconds from epoch UTC representing the end of the window for the usage data being reported
	IntervalEnd int64 `json:"end,omitempty" mapstructure:"-"`

	// The id of the IBM Software Central or Red Hat Marketplace account that usage is being reported for.
	AccountID string `json:"accountId,omitempty" mapstructure:"-"`

	// An array of usage objects.
	MeasuredUsage []MeasuredUsage `json:"measuredUsage" mapstructure:"-"`

	// --- Additional Properties ---
	// mapstructure tag should match the prometheus label to facilitate mapstructure.Decode()

	// The id of the IBM Software Central or Red Hat Marketplace subscription for the product that usage is being reported for.
	SubscriptionId                 string `json:"subscriptionId,omitempty" mapstructure:"subscriptionId"`
	Source                         string `json:"source,omitempty" mapstructure:"source"`
	SourceSaas                     string `json:"sourceSaas,omitempty" mapstructure:"sourceSaas"`
	AccountIdSaas                  string `json:"accountIdSaas,omitempty" mapstructure:"accountIdSaas"`
	SubscriptionIdSaas             string `json:"subscriptionIdSaas,omitempty" mapstructure:"subscriptionIdSaas"`
	ProductType                    string `json:"productType,omitempty" mapstructure:"productType"`
	LicensePartNumber              string `json:"licensePartNumber,omitempty" mapstructure:"licensePartNumber"`
	ProductId                      string `json:"productId,omitempty" mapstructure:"productId"`
	SapEntitlementLine             string `json:"sapEntitlementLine,omitempty" mapstructure:"sapEntitlementLine"`
	ProductName                    string `json:"productName,omitempty" mapstructure:"productName"`
	ParentProductId                string `json:"parentProductId,omitempty" mapstructure:"parentProductId"`
	ParentProductName              string `json:"parentProductName,omitempty" mapstructure:"parentProductName"`
	ParentMetricId                 string `json:"parentMetricId,omitempty" mapstructure:"parentMetricId"`
	TopLevelProductId              string `json:"topLevelProductId,omitempty" mapstructure:"topLevelProductId"`
	TopLevelProductName            string `json:"topLevelProductName,omitempty" mapstructure:"topLevelProductName"`
	TopLevelProductMetricId        string `json:"topLevelProductMetricId,omitempty" mapstructure:"topLevelProductMetricId"`
	DswOfferAccountingSystemCode   string `json:"dswOfferAccountingSystemCode,omitempty" mapstructure:"dswOfferAccountingSystemCode"`
	DswSubscriptionAgreementNumber string `json:"dswSubscriptionAgreementNumber,omitempty" mapstructure:"dswSubscriptionAgreementNumber"`
	SsmSubscriptionId              string `json:"ssmSubscriptionId,omitempty" mapstructure:"ssmSubscriptionId"`
	ICN                            string `json:"ICN,omitempty" mapstructure:"ICN"`
	Group                          string `json:"group,omitempty" mapstructure:"group"`
	GroupName                      string `json:"groupName,omitempty" mapstructure:"groupName"`
	Kind                           string `json:"kind,omitempty" mapstructure:"kind"`
}

type MeasuredUsage struct {

	// The ID of the metric being reported. If multiple objects inside measuredUsage have the same metricId for the same product the complete event will be rejected and NOT processed.
	MetricID string `json:"metricId" mapstructure:"-"`

	// The recorded usage value for the time window.
	Value float64 `json:"value" mapstructure:"-"`

	// --- Additional Properties ---
	NamespacesLabels []NamespaceLabels `json:"namespace,omitempty" mapstructure:"-"`

	// mapstructure tag should match the prometheus label to facilitate mapstructure.Decode()
	MeterDefNamespace      string `json:"meter_def_namespace,omitempty" mapstructure:"meter_def_namespace"`
	MeterDefName           string `json:"meter_def_name,omitempty" mapstructure:"meter_def_name"`
	MetricType             string `json:"metricType,omitempty" mapstructure:"metricType"`
	MetricAggregationType  string `json:"metricAggregationType,omitempty" mapstructure:"metricAggregationType"`
	MeasuredMetricId       string `json:"measuredMetricId,omitempty" mapstructure:"measuredMetricId"`
	ProductConversionRatio string `json:"productConversionRatio,omitempty" mapstructure:"productConversionRatio"`
	MeasuredValue          string `json:"measuredValue,omitempty" mapstructure:"measuredValue"`
	ClusterId              string `json:"clusterId,omitempty" mapstructure:"clusterId"`
	Hostname               string `json:"hostname,omitempty" mapstructure:"hostname"`
	Pod                    string `json:"pod,omitempty" mapstructure:"pod"`
	PlatformId             string `json:"platformId,omitempty" mapstructure:"platformId"`
	Crn                    string `json:"crn,omitempty" mapstructure:"crn"`
	IsViewable             string `json:"isViewable,omitempty" mapstructure:"isViewable"`
	CalculateSummary       string `json:"calculateSummary,omitempty" mapstructure:"calculateSummary"`

	// An array of kubernetes objects. Typically relating to infrastructure consisting of nodes, clusterversions, namespaces
	KubernetesResources []json.RawMessage `json:"k8sResources,omitempty" mapstructure:"-"`
}

type NamespaceLabels struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}
