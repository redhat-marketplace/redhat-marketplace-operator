package common

import (
	"encoding/json"
	"time"

	"emperror.dev/errors"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MeterDefPrometheusLabels struct {
	UID               string `json:"meter_definition_uid"`
	MeterDefName      string `json:"meter_def_name"`
	MeterDefNamespace string `json:"meter_def_namespace"`

	WorkloadName           string           `json:"workload_name"`
	WorkloadType           string           `json:"workload_type"`
	MeterGroup             string           `json:"meter_def_group"`
	MeterKind              string           `json:"meter_def_kind"`
	Metric                 string           `json:"metric_label"`
	MetricAggregation      string           `json:"metric_aggregation,omitempty"`
	MetricPeriod           *metav1.Duration `json:"metric_period,omitempty"`
	MetricQuery            string           `json:"metric_query"`
	MetricWithout          JSONArray        `json:"metric_without"`
	MetricGroupBy          JSONArray        `json:"metric_group_by,omitempty"`
	MetricAdditionalLabels AdditionalLabels `json:"metric_additional_labels,omitempty"`
}

func (m *MeterDefPrometheusLabels) Defaults() {
	if m.MetricPeriod == nil {
		m.MetricPeriod = &metav1.Duration{Duration: time.Hour}
	}

	if m.MetricAggregation == "" {
		m.MetricAggregation = "sum"
	}
}

func (m *MeterDefPrometheusLabels) FromLabels(labels interface{}) error {
	labelsObj := map[string]interface{}{}

	switch iMap := labels.(type) {
	case model.Metric:
		for v, k := range iMap {
			labelsObj[string(k)] = string(v)
		}
	case map[string]string:
		for v, k := range iMap {
			labelsObj[string(k)] = string(v)
		}
	default:
		return errors.New("unknown type")
	}

	data, err := json.Marshal(labelsObj)

	if err != nil {
		return err
	}

	err = json.Unmarshal(data, m)

	if err != nil {
		return err
	}

	m.Defaults()

	return nil
}

type JSONArray []string

func (a *JSONArray) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, a); err != nil {
		return err
	}
	return nil
}

func (a JSONArray) MarshalJSON() ([]byte, error) {
	return json.Marshal(a)
}

type AdditionalLabels struct {
	MeterDescription   string `json:"meter_description,omitempty"`
	ValueLabelOverride string `json:"value_label_override,omitempty"`
	DateLabelOverride  string `json:"date_label_override,omitempty"`
}

func (a *AdditionalLabels) UnmashalJSON(b []byte) error {
	if err := json.Unmarshal(b, a); err != nil {
		return err
	}
	return nil
}

func (a AdditionalLabels) MarshalJSON() ([]byte, error) {
	return json.Marshal(a)
}

type ToPrometheusLabels interface {
	ToPrometheusLabels() ([]map[string]string, error)
}
