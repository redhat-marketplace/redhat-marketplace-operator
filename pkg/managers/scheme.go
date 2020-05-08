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

package managers

import (
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/google/wire"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
)

type OpsSrcSchemeDefinition SchemeDefinition
type MonitoringSchemeDefinition SchemeDefinition
type OlmV1SchemeDefinition SchemeDefinition
type OlmV1Alpha1SchemeDefinition SchemeDefinition

func ProvideOpsSrcScheme() *OpsSrcSchemeDefinition {
	return &OpsSrcSchemeDefinition{
		Name:        "opsrcv1",
		AddToScheme: opsrcv1.SchemeBuilder.AddToScheme,
	}
}

func ProvideMonitoringScheme() *MonitoringSchemeDefinition {
	return &MonitoringSchemeDefinition{
		Name:        "monitoringv1",
		AddToScheme: monitoringv1.AddToScheme,
	}
}

func ProvideOLMV1Scheme() *OlmV1SchemeDefinition {
	return &OlmV1SchemeDefinition{
		Name:        "olmv1",
		AddToScheme: olmv1.AddToScheme,
	}
}

func ProvideOLMV1Alpha1Scheme() *OlmV1Alpha1SchemeDefinition {
	return &OlmV1Alpha1SchemeDefinition{
		Name:        "olmv1alpha1",
		AddToScheme: olmv1alpha1.AddToScheme,
	}
}

var SchemeDefinitions = wire.NewSet(
	ProvideOpsSrcScheme,
	ProvideMonitoringScheme,
	ProvideOLMV1Scheme,
	ProvideOLMV1Alpha1Scheme,
)
