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

package reporter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"text/template"

	"emperror.dev/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

type PromQueryArgs struct {
	Type          v1beta1.WorkloadType
	MeterDef      types.NamespacedName
	Metric        string
	Query         string
	Start, End    time.Time
	Step          time.Duration
	AggregateFunc string
	GroupBy       []string
	Without       []string
}

type PromQuery struct {
	*PromQueryArgs
}

func NewPromQuery(
	args *PromQueryArgs,
) *PromQuery {
	pq := &PromQuery{PromQueryArgs: args}
	pq.defaulter()
	return pq
}

const TypeNotSupportedErr = errors.Sentinel("type is not supported")

func (q *PromQuery) typeNotSupportedError() error {
	err := errors.WithDetails(TypeNotSupportedErr, "type", string(q.Type))
	logger.Error(err, "type not supported", "type", string(q.Type))
	return err
}

func (q *PromQuery) makeLeftSide() string {
	switch q.Type {
	case v1beta1.WorkloadTypePVC:
		return fmt.Sprintf(`avg(meterdef_persistentvolumeclaim_info{meter_def_name="%v",meter_def_namespace="%v",phase="Bound"}) without (instance, container, endpoint, job, service)`, q.MeterDef.Name, q.MeterDef.Namespace)
	case v1beta1.WorkloadTypePod:
		return fmt.Sprintf(`avg(meterdef_pod_info{meter_def_name="%v",meter_def_namespace="%v"}) without (pod_uid, instance, container, endpoint, job, service)`, q.MeterDef.Name, q.MeterDef.Namespace)
	case v1beta1.WorkloadTypeService:
		return fmt.Sprintf(`avg(meterdef_service_info{meter_def_name="%v",meter_def_namespace="%v"}) without (pod_uid, instance, container, endpoint, job, pod)`, q.MeterDef.Name, q.MeterDef.Namespace)
	default:
		panic(q.typeNotSupportedError())
	}
}

func (q *PromQuery) defaulter() {
	q.defaultWithout()
	q.defaultGroupBy()
}

func (q *PromQuery) defaultWithout() {
	// we want to make sure
	switch q.Type {
	case v1beta1.WorkloadTypePVC:
		q.Without = append(q.Without, "instance")
	case v1beta1.WorkloadTypePod:
		q.Without = append(q.Without, "pod_ip", "instance", "image_id", "host_ip", "node")
	case v1beta1.WorkloadTypeService:
		q.Without = append(q.Without, "instance", "cluster_ip")
	default:
		panic(q.typeNotSupportedError())
	}
}

func (q *PromQuery) defaultGroupBy() {
	if len(q.GroupBy) != 0 {
		return
	}

	switch q.Type {
	case v1beta1.WorkloadTypePVC:
		q.GroupBy = []string{"persistentvolumeclaim", "namespace"}
	case v1beta1.WorkloadTypePod:
		q.GroupBy = []string{"pod", "namespace"}
	case v1beta1.WorkloadTypeService:
		q.GroupBy = []string{"service", "namespace"}
	default:
		panic(q.typeNotSupportedError())
	}
}

const resultQueryTemplateStr = `
{{- .AggregateFunc }} by ({{ .GroupBy }}) ({{ .LeftSide }} * on({{ .GroupBy }}) group_right {{ .Query }}) * on({{ .GroupBy }}) group_right group without({{ .Without }}) ({{ .Query }})`

var resultQueryTemplate *template.Template = utils.Must(func() (interface{}, error) {
	return template.New("resultQuery").Parse(resultQueryTemplateStr)
}).(*template.Template)

type ResultQueryArgs struct {
	AggregateFunc, GroupBy, LeftSide, Without, Query string
}

func (q *PromQuery) GetQueryArgs() ResultQueryArgs {
	return ResultQueryArgs{
		Query:         q.Query,
		AggregateFunc: q.AggregateFunc,
		GroupBy:       strings.Join(q.GroupBy, ","),
		LeftSide:      q.makeLeftSide(),
		Without:       strings.Join(q.Without, ","),
	}
}

func (q *PromQuery) Print() (string, error) {
	var buf strings.Builder
	err := resultQueryTemplate.Execute(&buf, q.GetQueryArgs())
	return buf.String(), err
}

func (r *MarketplaceReporter) queryRange(query *PromQuery) (model.Value, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	timeRange := v1.Range{
		Start: query.Start,
		End:   query.End,
		Step:  query.Step,
	}

	q, err := query.Print()

	if err != nil {
		return nil, nil, err
	}

	logger.Info("executing query", "query", q)

	result, warnings, err := r.api.QueryRange(ctx, q, timeRange)

	if err != nil {
		logger.Error(err, "querying prometheus", "warnings", warnings)
		return nil, warnings, toError(err)
	}
	if len(warnings) > 0 {
		logger.Info("warnings", "warnings", warnings)
	}

	return result, warnings, nil
}

var ClientError = errors.Sentinel("clientError")
var ClientErrorUnauthorized = errors.Sentinel("clientError: Unauthorized")
var ServerError = errors.Sentinel("serverError")

func toError(err error) error {
	if v, ok := err.(*v1.Error); ok {
		if v.Type == v1.ErrClient {
			if strings.Contains(strings.ToLower(v.Msg), "unauthorized") {
				return errors.Combine(errors.WithStack(ClientErrorUnauthorized), err)
			}

			return errors.Combine(errors.WithStack(ClientError), err)
		}

		return errors.Combine(errors.WithStack(ServerError), err)
	}

	return err
}

type MeterDefinitionQuery struct {
	Start, End time.Time
	Step       time.Duration
}

// Returns a set of elements without duplicates
// Ignore labels such that a pod restart, meterdefinition recreate, or other labels do not generate a new unique element
// Use max over time to get the meter definition most prevalent for the hour
const meterDefinitionQueryStr = `max_over_time(((meterdef_metric_label_info{} + ignoring(container, endpoint, instance, job, meter_definition_uid, pod, service) meterdef_metric_label_info{}) or on() vector(0))[{{ .Step }}:{{ .Step }}])`

var meterDefinitionQueryTemplate *template.Template = utils.Must(func() (interface{}, error) {
	return template.New("meterDefinitionQuery").Parse(meterDefinitionQueryStr)
}).(*template.Template)

func (q *MeterDefinitionQuery) Print() (string, error) {
	var buf strings.Builder
	err := meterDefinitionQueryTemplate.Execute(&buf, q)
	return buf.String(), err
}

func (r *MarketplaceReporter) queryMeterDefinitions(query *MeterDefinitionQuery) (model.Value, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	timeRange := v1.Range{
		Start: query.Start,
		End:   query.End,
		Step:  query.Step,
	}

	q, err := query.Print()

	if err != nil {
		logger.Error(err, "error with query")
		return nil, nil, err
	}

	logger.Info("executing query", "query", q)
	result, warnings, err := r.api.QueryRange(ctx, q, timeRange)

	if err != nil {
		logger.Error(err, "querying prometheus", "warnings", warnings)
		return nil, warnings, toError(err)
	}
	if len(warnings) > 0 {
		logger.Info("warnings", "warnings", warnings)
	}

	return result, warnings, nil
}
