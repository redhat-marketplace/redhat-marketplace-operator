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

	//MeterDefinitionCatlaogServer holds feature flags for the meterdefinition catalog server
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Enable Razee deployment?"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	// +optional
	MeterDefinitionCatalogServer MeterDefinitionCatalogServer `json:"meterDefinitionCatalogServer,omitempty"`
}

// MeterDefinitionCatalogServer represents a list of features that can be enabled or disabled for the Meterdefinition Catalog Server.
// +kubebuilder:object:generate:=true
type MeterDefinitionCatalogServer struct {
	// SyncCommunityMeterDefinitions represents the enablement of the Meterdefinition Catalog Server with a directive to sync community meterdefinitions from the meterdefinition catalog, defaults to true when not set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Enable Community Meterdefinition Catalog Server?"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	// +optional
	SyncCommunityMeterDefinitions *bool `json:"syncCommunityMeterDefinitions,omitempty"`

	// SyncSystemMeterDefinitions represents the enablement of the Meterdefinition Catalog Server with a directive to sync system meterdefinitions from the meterdefinition catalog, defaults to true when not set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Enable License Usage Metering?"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	// +optional
	SyncSystemMeterDefinitions *bool `json:"syncSystemMeterDefinitions,omitempty"`

	// DeployFileServer controls whether the deploymentconfig controller will deploy the resources needed 
	// to create the Meterdefinition Catalog Server. The Catalog Server will look for changes to an image repository and pull down the latest
	// Image when a change is detected.
	// Setting DeployFileServer to "true" wil install a DeploymentConfig, ImageStream, and Service. 
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Enable License Usage Metering?"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	// +optional
	DeployMeterDefinitionCatalogServer *bool `json:"deployMeterDefinitionCatalogServer,omitempty"`
}
