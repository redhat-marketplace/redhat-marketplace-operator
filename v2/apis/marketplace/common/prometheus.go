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
	"reflect"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/mitchellh/mapstructure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MeterDefPrometheusLabels struct {
	UID               string `json:"meter_definition_uid" mapstructure:"meter_definition_uid"`
	MeterDefName      string `json:"name" mapstructure:"name"`
	MeterDefNamespace string `json:"namespace" mapstructure:"namespace"`

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

	MeterDescription   string `json:"meter_description,omitempty" mapstructure:"meter_description,omitempty" template:""`
	ValueLabelOverride string `json:"value_label_override,omitempty" mapstructure:"value_label_override,omitempty" template:""`
	DateLabelOverride  string `json:"date_label_override,omitempty" mapstructure:"date_label_override,omitempty" template:""`
}

func convertToString(v interface{}) (string, error) {
	val := reflect.ValueOf(v)

	if val.Kind() == reflect.Interface {
		if val.IsNil() {
			return "", errors.New("value is nil")
		}

		elm := val.Elem()

		if elm.IsNil() {
			return "", errors.New("interface value is nil")
		}

		if elm.Kind() == reflect.Ptr && elm.Elem().Kind() == reflect.Ptr {
			val = elm
		}
	}

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.String:
		return val.String(), nil
	case reflect.Slice:
		fallthrough
	case reflect.Struct:
		fallthrough
	case reflect.Map:
		fallthrough
	case reflect.Interface:
		switch v.(type) {
		case fmt.Stringer:
			return v.(fmt.Stringer).String(), nil
		case map[string]interface{}:
			strbytes, err := json.Marshal(v)
			if err != nil {
				return "", errors.Wrap(err, "value failed json")
			}
			return string(strbytes), nil
		case json.Marshaler:
			strbytes, err := v.(json.Marshaler).MarshalJSON()
			if err != nil {
				return "", errors.Wrap(err, "value failed json")
			}
			return string(strbytes), nil
		default:
			return "", errors.NewWithDetails("not stringable", "type", reflect.TypeOf(v).String())
		}
	default:
		return "", errors.NewWithDetails("struct not defined for conversion", "kind", val.Kind().String())
	}
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
	labelsMap := map[string]interface{}{}
	err := mapstructure.Decode(m, &labelsMap)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{}
	for k, v := range labelsMap {
		if v == nil {
			continue
		}

		vstr, err := convertToString(v)

		if err != nil {
			return labels, err
		}

		if vstr != "" {
			labels[k] = vstr
		}
	}

	return labels, nil
}

func (m *MeterDefPrometheusLabels) FromLabels(labels interface{}) error {
	data, err := json.Marshal(labels)

	if err != nil {
		return err
	}

	newObj := MeterDefPrometheusLabels{}
	err = json.Unmarshal(data, &newObj)

	if err != nil {
		return err
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
	dur, err := time.ParseDuration(string(b))

	if err != nil {
		return err
	}

	a.Duration = dur
	return nil
}

func (a MetricPeriod) MarshalJSON() ([]byte, error) {
	str := a.Duration.String()
	return json.Marshal(str)
}

type JSONArray []string

var _ json.Marshaler = &JSONArray{}
var _ json.Unmarshaler = &JSONArray{}

func (a *JSONArray) UnmarshalJSON(b []byte) error {
	str, err := strconv.Unquote(string(b))

	if err != nil {
		return err
	}

	var j []string

	if err := json.Unmarshal([]byte(str), &j); err != nil {
		return err
	}

	*a = JSONArray(j)
	return nil
}

func (a JSONArray) MarshalJSON() ([]byte, error) {
	if a == nil {
		return []byte{}, nil
	}

	b, err := json.Marshal([]string(a))
	if err != nil {
		return b, err
	}

	return b, nil
}

type ToPrometheusLabels interface {
	ToPrometheusLabels() ([]map[string]string, error)
}
