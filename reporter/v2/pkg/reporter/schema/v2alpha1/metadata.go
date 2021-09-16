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

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
)

type SourceMetadata struct {
	RhmClusterID   string                   `json:"rhmClusterId" mapstructure:"rhmClusterId"`
	RhmAccountID   string                   `json:"rhmAccountId" mapstructure:"rhmAccountId"`
	RhmEnvironment common.ReportEnvironment `json:"rhmEnvironment,omitempty" mapstructure:"rhmEnvironment,omitempty"`
	Version        string                   `json:"version,omitempty" mapstructure:"version,omitempty"`
	ReportVersion  string                   `json:"reportVersion,omitempty"`
}

type Manifest struct {
	Version string `json:"version"`
	Type    string `json:"type"`
}

const AccountMetrics = "accountMetrics"