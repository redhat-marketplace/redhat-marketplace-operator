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

package common

import (
	"encoding/json"
	"fmt"
	"time"

	"emperror.dev/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MeterDefPrometheusLabels struct {
	UID               string `json:"meter_definition_uid" mapstructure:"meter_definition_uid"`
	MeterDefName      string `json:"name" mapstructure:"name"`
	MeterDefNamespace string `json:"namespace" mapstructure:"namespace"`

	// Deprecated: metric is now the primary name
	WorkloadName      string        `json:"workload_name" mapstructure:"workload_name"`
	WorkloadType      string        `json:"workload_type" mapstructure:"workload_type"`
	MeterGroup        string        `json:"meter_group" mapstructure:"meter_group" template:""`
	MeterKind         string        `json:"meter_kind" mapstructure:"meter_kind" template:""`
	Metric            string        `json:"metric_label" mapstructure:"metric_label" template:""`
	MetricAggregation string        `json:"metric_aggregation,omitempty" mapstructure:"metric_aggregation"`
	MetricPeriod      *MetricPeriod `json:"metric_period,omitempty" mapstructure:"metric_period"`
	MetricQuery       string        `json:"metric_query" mapstructure:"metric_query"`
	MetricWithout     JSONArray     `json:"metric_without" mapstructure:"metric_without"`
	MetricGroupBy     JSONArray     `json:"metric_group_by,omitempty" mapstructure:"metric_group_by"`

	DisplayName        string `json:"display_name,omitempty" mapstructure:"display_name,omitempty" template:""`
	MeterDescription   string `json:"meter_description,omitempty" mapstructure:"meter_description,omitempty" template:""`
	ValueLabelOverride string `json:"value_label_override,omitempty" mapstructure:"value_label_override,omitempty" template:""`
	DateLabelOverride  string `json:"date_label_override,omitempty" mapstructure:"date_label_override,omitempty" template:""`
}

func (m *MeterDefPrometheusLabels) Defaults() {
	if m.MetricPeriod == nil {
		m.MetricPeriod = &MetricPeriod{Duration: time.Hour}
	}

	if m.MetricAggregation == "" {
		m.MetricAggregation = "sum"
	}
}

func (m *MeterDefPrometheusLabels) ToLabels() (map[string]string, error) {
	bytes, err := json.Marshal(m)

	if err != nil {
		return nil, err
	}

	labelsMap := map[string]string{}
	err = json.Unmarshal(bytes, &labelsMap)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return labelsMap, nil
}

func (m *MeterDefPrometheusLabels) FromLabels(labels interface{}) error {
	data, err := json.Marshal(labels)

	if err != nil {
		return err
	}

	newObj := MeterDefPrometheusLabels{}
	err = json.Unmarshal(data, &newObj)

	if err != nil {
		return errors.WithStack(err)
	}

	newObj.Defaults()

	*m = newObj

	return nil
}

type MetricPeriod metav1.Duration

var _ fmt.Stringer = &MetricPeriod{}
var _ json.Marshaler = &MetricPeriod{}
var _ json.Unmarshaler = &MetricPeriod{}

func (a *MetricPeriod) UnmarshalJSON(b []byte) error {
	var val string
	err := json.Unmarshal(b, &val)
	if err != nil {
		return err
	}

	dur, err := time.ParseDuration(val)

	if err != nil {
		return errors.WithStack(err)
	}

	a.Duration = dur
	return nil
}

func (a MetricPeriod) MarshalJSON() ([]byte, error) {
	str := a.Duration.String()
	return json.Marshal(str)
}

type JSONArray []string

func (a *JSONArray) UnmarshalText(b []byte) error {
	str := string(b)

	if len(str) == 0 {
		*a = JSONArray{}
		return nil
	}

	var j []string

	if err := json.Unmarshal([]byte(str), &j); err != nil {
		return errors.WithStack(err)
	}

	*a = JSONArray(j)
	return nil
}

func (a JSONArray) MarshalText() ([]byte, error) {
	if a == nil {
		return []byte{}, nil
	}

	b, err := json.Marshal([]string(a))
	if err != nil {
		return b, errors.WithStack(err)
	}

	return b, nil
}

type ToPrometheusLabels interface {
	ToPrometheusLabels() ([]map[string]string, error)
}
