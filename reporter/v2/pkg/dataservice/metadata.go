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

package dataservice

import (
	mapstructure "github.com/go-viper/mapstructure/v2"
	"k8s.io/apimachinery/pkg/types"
)

type MeterReportMetadata struct {
	ReportName      string `mapstructure:"reportName"`
	ReportNamespace string `mapstructure:"reportNamespace"`
}

func (m MeterReportMetadata) Map() (out map[string]string, err error) {
	err = mapstructure.Decode(m, &out)
	return
}

func (m *MeterReportMetadata) From(in map[string]string) error {
	return mapstructure.Decode(in, m)
}

func (m *MeterReportMetadata) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      m.ReportName,
		Namespace: m.ReportNamespace,
	}
}

func (m *MeterReportMetadata) IsEmpty() bool {
	return m.ReportName == "" || m.ReportNamespace == ""
}
