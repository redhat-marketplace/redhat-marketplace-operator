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

package common

// Feature represents a list of features that can be enabled or disabled.
// +kubebuilder:object:generate:=true
type Features struct {

	// Deployment represents the enablement of the razee deployment, defaults to true when not set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Enable Razee deployment?"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	// +optional
	Deployment *bool `json:"deployment,omitempty"`

	// Registration represents the enablement of the registration watchkeeper deployment, defaults to true when not set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Enable Watchkeeper deployment?"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	// +optional
	Registration *bool `json:"registration,omitempty"`

	// DeploymentConfig represents the enablement of the Community Catalog DeploymentConfig, defaults to true when not set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Enable Community Catalog DeploymentConfig?"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	// +optional
	DeploymentConfig *bool `json:"deploymentConfig,omitempty"`

	// LicenseUsageMetering represents the enablement of the usage metering for licensed products, defaults to true when not set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Enable License Usage Metering?"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	// +optional
	LicenseUsageMetering *bool `json:"licenseUsageMetering,omitempty"`
}
