package common

import (
	"encoding/json"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/mitchellh/mapstructure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MeterDefPrometheusLabels struct {
	UID               string `json:"meter_definition_uid" mapstructure:"meter_definition_uid"`
	MeterDefName      string `json:"meter_def_name" mapstructure:"meter_def_name"`
	MeterDefNamespace string `json:"meter_def_namespace" mapstructure:"meter_def_namespace"`

	WorkloadName      string        `json:"workload_name" mapstructure:"workload_name"`
	WorkloadType      string        `json:"workload_type" mapstructure:"workload_type"`
	MeterGroup        string        `json:"meter_group" mapstructure:"meter_group"`
	MeterKind         string        `json:"meter_kind" mapstructure:"meter_kind"`
	Metric            string        `json:"metric_label" mapstructure:"metric_label"`
	MetricAggregation string        `json:"metric_aggregation,omitempty" mapstructure:"metric_aggregation"`
	MetricPeriod      *MetricPeriod `json:"metric_period,omitempty" mapstructure:"metric_period"`
	MetricQuery       string        `json:"metric_query" mapstructure:"metric_query"`
	MetricWithout     JSONArray     `json:"metric_without" mapstructure:"metric_without"`
	MetricGroupBy     JSONArray     `json:"metric_group_by,omitempty" mapstructure:"metric_group_by"`

	MeterDescription   string `json:"meter_description,omitempty" mapstructure:"meter_description,omitempty"`
	ValueLabelOverride string `json:"value_label_override,omitempty" mapstructure:"value_label_override,omitempty"`
	DateLabelOverride  string `json:"date_label_override,omitempty" mapstructure:"date_label_override,omitempty"`
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

		vstr := ""

		switch v.(type) {
		case map[string]interface{}:
			strbytes, err := json.Marshal(v.(map[string]interface{}))
			if err != nil {
				return nil, errors.Wrap(err, "value failed json")
			}
			vstr = string(strbytes)
		case json.Marshaler:
			strbytes, err := v.(json.Marshaler).MarshalJSON()
			if err != nil {
				return nil, errors.Wrap(err, "value failed json")
			}
			vstr = string(strbytes)
		case fmt.Stringer:
			vstr = v.(fmt.Stringer).String()
		case string:
			vstr = v.(string)
		case nil:
			vstr = ""
		default:
			vb, err := json.Marshal(v)

			if err != nil {
				return nil, err
			}

			vstr = string(vb)
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

func (p *MetricPeriod) String() string {
	if p == nil {
		return ""
	}

	return p.Duration.String()
}

type JSONArray []string

var _ json.Marshaler = &JSONArray{}
var _ json.Unmarshaler = &JSONArray{}

func (a *JSONArray) UnmarshalJSON(b []byte) error {
	var j []string
	if err := json.Unmarshal(b, &j); err != nil {
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
