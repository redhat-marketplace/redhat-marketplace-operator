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
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/cespare/xxhash"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	WorkloadTypePod     WorkloadType = "Pod"
	WorkloadTypeService WorkloadType = "Service"
	WorkloadTypePVC     WorkloadType = "PersistentVolumeClaim"

	MetricTypeBillable MetricType = "billable"
	MetricTypeLicense  MetricType = "license"
	MetricTypeAdoption MetricType = "adoption"
	MetricTypeEmpty    MetricType = ""
)

type MeterDefPrometheusLabels struct {
	UID               string `json:"meter_definition_uid" mapstructure:"meter_definition_uid"`
	MeterDefName      string `json:"name" mapstructure:"name"`
	MeterDefNamespace string `json:"namespace" mapstructure:"namespace"`

	// Deprecated: metric is now the primary name
	WorkloadName      string        `json:"workload_name" mapstructure:"workload_name" template:""`
	WorkloadType      WorkloadType  `json:"workload_type" mapstructure:"workload_type"`
	MeterGroup        string        `json:"meter_group" mapstructure:"meter_group" template:""`
	MeterKind         string        `json:"meter_kind" mapstructure:"meter_kind" template:""`
	Metric            string        `json:"metric_label" mapstructure:"metric_label" template:""`
	MetricAggregation string        `json:"metric_aggregation,omitempty" mapstructure:"metric_aggregation"`
	MetricPeriod      *MetricPeriod `json:"metric_period,omitempty" mapstructure:"metric_period"`
	MetricQuery       string        `json:"metric_query" mapstructure:"metric_query"`
	MetricWithout     JSONArray     `json:"metric_without" mapstructure:"metric_without"`
	MetricGroupBy     JSONArray     `json:"metric_group_by,omitempty" mapstructure:"metric_group_by"`
	MetricType        MetricType    `json:"metric_type,omitempty" mapstructure:"metric_type"`

	ResourceName      string `json:"resource_name,omitempty"`
	ResourceNamespace string `json:"resource_namespace,omitempty"`

	Label string `json:"label,omitempty" mapstructure:"label,omitempty"  template:""`
	Unit  string `json:"unit,omitempty" mapstructure:"unit,omitempty"  template:""`

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

const justDateFormat = "2006-01-02"

func (m *MeterDefPrometheusLabels) PrintTemplate(
	values *ReportLabels,
	pair model.SamplePair,
) (*MeterDefPrometheusLabelsTemplated, error) {
	tpl, err := NewTemplate(m)

	if err != nil {
		return nil, err
	}

	localM := *m
	err = tpl.Execute(&localM, values)

	if err != nil {
		return nil, err
	}

	result := &MeterDefPrometheusLabelsTemplated{
		MeterDefPrometheusLabels: &localM,
	}

	// set resourceNamespace
	{
		namespace, ok := values.Label["namespace"]

		if ok {
			result.ResourceNamespace = namespace.(string)
		}
	}

	// set resourceName
	{
		var ok bool
		var objName interface{}

		switch m.WorkloadType {
		case WorkloadTypePVC:
			objName, ok = values.Label["persistentvolumeclaim"]
		case WorkloadTypePod:
			objName, ok = values.Label["pod"]
		case WorkloadTypeService:
			objName, ok = values.Label["service"]
		}

		if ok {
			result.ResourceName = objName.(string)
		}
	}

	result.LabelMap = values.Label

	// set result
	if result.Label == "" {
		result.Label = result.Metric
	}

	// set value
	value := pair.Value.String()
	if result.ValueLabelOverride != "" {
		value = result.ValueLabelOverride
	}

	result.Value = value

	// set intervals
	intervalStart := pair.Timestamp.Time().UTC()
	intervalEnd := pair.Timestamp.Add(result.MetricPeriod.Duration).Time().UTC()

	if m.DateLabelOverride != "" {
		t, err := time.Parse(time.RFC3339, result.DateLabelOverride)

		if err != nil {
			t2, err2 := time.Parse(justDateFormat, result.DateLabelOverride)

			if err2 != nil {
				return nil, errors.Combine(err, err2)
			}

			t = t2
		}

		intervalStart = t
		intervalEnd = t.Add(result.MetricPeriod.Duration)
	}

	result.IntervalStart = intervalStart
	result.IntervalEnd = intervalEnd

	return result, nil
}

// MeterDefPrometheusLabelsTemplated is the values of a meter definition
// templated and with values set ready to be added.
type MeterDefPrometheusLabelsTemplated struct {
	*MeterDefPrometheusLabels

	IntervalStart, IntervalEnd time.Time
	Value                      string
	LabelMap                   map[string]interface{}

	hash string
}

func (m *MeterDefPrometheusLabelsTemplated) Hash() string {
	if m.hash == "" {
		hash := xxhash.New()
		hash.Write([]byte(m.IntervalStart.UTC().Format(time.RFC3339)))
		hash.Write([]byte(m.IntervalEnd.UTC().Format(time.RFC3339)))
		hash.Write([]byte(m.MeterGroup))
		hash.Write([]byte(m.MeterKind))
		hash.Write([]byte(m.Metric))
		hash.Write([]byte(m.ResourceNamespace))
		hash.Write([]byte(m.ResourceName))
		hash.Write([]byte(m.Unit))
		m.hash = fmt.Sprintf("%x", hash.Sum64())
	}

	return m.hash
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

// Result is a result of a query defined on the meterdefinition.
// This will generate data for the previous hour on whichever workload you specify.
// This will allow you to check whether a query is working as intended.
// +k8s:openapi-gen=true
// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
// +kubebuilder:object:generate:=true
type Result struct {
	// MetricName is the identifier that you will use to identify your query
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	MetricName string `json:"metricName,omitempty"`

	// Query is the compiled query that is given to Prometheus
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Query string `json:"query,omitempty"`

	// Values are the results of the query
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Values []ResultValues `json:"values,omitempty"`
}

// ResultValues will hold the results of the prometheus query
// +k8s:openapi-gen=true
// +kubebuilder:object:generate:=true
type ResultValues struct {
	Timestamp int64             `json:"timestamp"`
	Value     string            `json:"value"`
	Labels    map[string]string `json:"labels"`
}

// Target is used by meterbase as a list of prometheus activeTargets with failed health, without DiscoveredLabels
// +k8s:openapi-gen=true
// +kubebuilder:object:generate:=true
type Target struct {
	Labels     model.LabelSet            `json:"labels"`
	ScrapeURL  string                    `json:"scrapeUrl"`
	LastError  string                    `json:"lastError"`
	LastScrape string                    `json:"lastScrape"`
	Health     prometheusv1.HealthStatus `json:"health"`
}

type MetricType string

func (a *MetricType) UnmarshalJSON(b []byte) error {
	str, err := strconv.Unquote(string(b))

	if err != nil {
		return err
	}

	*a = MetricType(str)
	return nil
}

func (a MetricType) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(string(a))), nil
}

func (a MetricType) String() string {
	return string(a)
}

type WorkloadType string

func (a *WorkloadType) UnmarshalJSON(b []byte) error {
	str, err := strconv.Unquote(string(b))

	if err != nil {
		return err
	}

	*a = WorkloadType(str)
	return nil
}

func (a WorkloadType) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(string(a))), nil
}
