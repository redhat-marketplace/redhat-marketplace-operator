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

package prometheus

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"emperror.dev/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = logf.Log.WithName("reporter")

type PromQueryArgs struct {
	Type          v1alpha1.WorkloadType
	MeterDef      types.NamespacedName
	Metric        string
	Query         string
	Start, End    time.Time
	Step          time.Duration
	AggregateFunc string
	GroupBy       []string
	Without       []string
	Time          string
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

type PrometheusAPI struct {
	v1.API
}

//TODO: what's the difference between these ?
// func (q *PromQuery) makeLeftSide() string {
// 	switch q.Type {
// 	case v1alpha1.WorkloadTypePVC:
// 		return fmt.Sprintf(`avg(meterdef_persistentvolumeclaim_info{meter_def_name="%v",meter_def_namespace="%v",phase="Bound"}) without (instance, container, endpoint, job, service)`, q.MeterDef.Name, q.MeterDef.Namespace)
// 	case v1alpha1.WorkloadTypePod:
// 		return fmt.Sprintf(`avg(meterdef_pod_info{meter_def_name="%v",meter_def_namespace="%v"}) without (pod_uid, instance, container, endpoint, job, service)`, q.MeterDef.Name, q.MeterDef.Namespace)
// 	case v1alpha1.WorkloadTypeService:
// 		// Service and service monitor are handled the same
// 		fallthrough
// 	case v1alpha1.WorkloadTypeServiceMonitor:
// 		return fmt.Sprintf(`avg(meterdef_service_info{meter_def_name="%v",meter_def_namespace="%v"}) without (pod_uid, instance, container, endpoint, job, pod)`, q.MeterDef.Name, q.MeterDef.Namespace)
// 	default:
// 		panic(q.typeNotSupportedError())
// 	}
// }


func (q *PromQuery) makeLeftSide() string {
	switch q.Type {
	case v1alpha1.WorkloadTypePVC:
		return fmt.Sprintf(`avg(meterdef_persistentvolumeclaim_info{meter_def_name="%v",meter_def_namespace="%v",phase="Bound"}) without (instance, container, endpoint, job, service)`, q.MeterDef.Name, q.MeterDef.Namespace)
	case v1alpha1.WorkloadTypePod:
		return fmt.Sprintf(`avg(meterdef_pod_info{meter_def_name="%v",meter_def_namespace="%v"}) without (pod_uid, instance, container, endpoint, job, service)`, q.MeterDef.Name, q.MeterDef.Namespace)
	case v1alpha1.WorkloadTypeService:
		// Service and service monitor are handled the same
		fallthrough
	case v1alpha1.WorkloadTypeServiceMonitor:
		return fmt.Sprintf(`avg(meterdef_service_info{meter_def_name="%v",meter_def_namespace="%v"}) without (pod_uid, instance, container, endpoint, job, pod)`, q.MeterDef.Name, q.MeterDef.Namespace)
	default:
		return "NOTSUPPORTED"
	}
}

func (q *PromQuery) makeJoin() string {
	switch q.Type {
	case v1alpha1.WorkloadTypePVC:
		return "* on(persistentvolumeclaim,namespace) group_right"
	case v1alpha1.WorkloadTypePod:
		return "* on(pod,namespace) group_right"
	case v1alpha1.WorkloadTypeService:
		fallthrough
	case v1alpha1.WorkloadTypeServiceMonitor:
		return "* on(service,namespace) group_right"
	default:
		return "NOTSUPPORTED"
	}
}

func (q *PromQuery) makeAggregateBy() string {
	switch q.Type {
	case v1alpha1.WorkloadTypePVC:
		return fmt.Sprintf(`%v by (persistentvolumeclaim,namespace)`, q.AggregateFunc)
	case v1alpha1.WorkloadTypePod:
		return fmt.Sprintf(`%v by (pod,namespace)`, q.AggregateFunc)
	case v1alpha1.WorkloadTypeService:
		fallthrough
	case v1alpha1.WorkloadTypeServiceMonitor:
		return fmt.Sprintf(`%v by (service,namespace)`, q.AggregateFunc)
	default:
		return "NOTSUPPORTED"
	}
}

func (q *PromQuery) defaulter() {
	q.defaultWithout()
	q.defaultGroupBy()
}

func (q *PromQuery) defaultWithout() {
	// we want to make sure
	switch q.Type {
	case v1alpha1.WorkloadTypePVC:
		q.Without = append(q.Without, "instance")
	case v1alpha1.WorkloadTypePod:
		q.Without = append(q.Without, "pod_ip", "instance", "image_id", "host_ip", "node")
	case v1alpha1.WorkloadTypeService:
		fallthrough
	case v1alpha1.WorkloadTypeServiceMonitor:
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
	case v1alpha1.WorkloadTypePVC:
		q.GroupBy = []string{"persistentvolumeclaim", "namespace"}
	case v1alpha1.WorkloadTypePod:
		q.GroupBy = []string{"pod", "namespace"}
	case v1alpha1.WorkloadTypeService:
		fallthrough
	case v1alpha1.WorkloadTypeServiceMonitor:
		q.GroupBy = []string{"service", "namespace"}
	default:
		panic(q.typeNotSupportedError())
	}
}

func (q *PromQuery) String() string {
	aggregate := q.makeAggregateBy()
	leftSide := q.makeLeftSide()
	join := q.makeJoin()

	var query string
	if q.Query != "" {
		query = q.Query
	} else {
		query = fmt.Sprintf("%s{}", q.Metric)
	}

	return fmt.Sprintf(
		`%v (%v %v %v)`, aggregate, leftSide, join, query,
	)
}

func (p *PrometheusAPI)ReportQuery(query *PromQuery) (model.Value, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	timeRange := v1.Range{
		Start: query.Start,
		End:   query.End,
		Step:  query.Step,
	}

	result, warnings, err := p.QueryRange(ctx, query.String(), timeRange)

	if err != nil {
		logger.Error(err, "querying prometheus", "warnings", warnings)
		return nil, warnings, toError(err)
	}
	if len(warnings) > 0 {
		logger.Info("warnings", "warnings", warnings)
	}

	return result, warnings, nil
}

//TODO: being used in the reporter
func ReportQueryFromAPI(query *PromQuery, promApi v1.API) (model.Value, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	timeRange := v1.Range{
		Start: query.Start,
		End:   query.End,
		Step:  query.Step,
	}

	result, warnings, err := promApi.QueryRange(ctx, query.String(), timeRange)

	if err != nil {
		logger.Error(err, "querying prometheus", "warnings", warnings)
		return nil, warnings, toError(err)
	}
	if len(warnings) > 0 {
		logger.Info("warnings", "warnings", warnings)
	}

	return result, warnings, nil
}

var ClientError = errors.New("clientError")
var ClientErrorUnauthorized = errors.New("clientError: Unauthorized")
var ServerError = errors.New("serverError")

func toError(err error) error {
	if v, ok := err.(*v1.Error); ok {
		if v.Type == v1.ErrClient {
			if strings.Contains(strings.ToLower(v.Msg), "unauthorized") {
				return errors.Combine(ClientErrorUnauthorized, err)
			}

			return errors.Combine(ClientError, err)
		}

		return errors.Combine(ServerError, err)
	}

	return err
}

const TypeNotSupportedErr = errors.Sentinel("type is not supported")

func (q *PromQuery) typeNotSupportedError() error {
	err := errors.WithDetails(TypeNotSupportedErr, "type", string(q.Type))
	logger.Error(err, "type not supported", "type", string(q.Type))
	return err
}

const resultQueryTemplateStr = `
{{- .AggregateFunc }} by ({{ .GroupBy }}) ({{ .LeftSide }} * on({{ .GroupBy }}) group_right {{ .Query }}) * on({{ .GroupBy }}) group_right group without({{ .Without }}) ({{ .Query }})`

var resultQueryTemplate *template.Template = utils.Must(func() (interface{}, error) {
	return template.New("resultQuery").Parse(resultQueryTemplateStr)
}).(*template.Template)

type ResultQueryArgs struct {
	AggregateFunc, GroupBy, LeftSide, Without, Query string
}

func (p *PromQuery) GetQueryArgs() ResultQueryArgs {
	return ResultQueryArgs{
		Query:         p.Query,
		AggregateFunc: p.AggregateFunc,
		GroupBy:       strings.Join(p.GroupBy, ","),
		LeftSide:      p.makeLeftSide(),
		Without:       strings.Join(p.Without, ","),
	}
}

func (q *PromQuery) Print() (string, error) {
	var buf strings.Builder
	err := resultQueryTemplate.Execute(&buf, q.GetQueryArgs())
	return buf.String(), err
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

func(p *PrometheusAPI) QueryMeterDefinitions(query *MeterDefinitionQuery) (model.Value, v1.Warnings, error) {
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
	// result, warnings, err := api.QueryRange(ctx, q, timeRange)
	result, warnings, err := p.QueryRange(ctx, q, timeRange)

	if err != nil {
		logger.Error(err, "querying prometheus", "warnings", warnings)
		return nil, warnings, toError(err)
	}
	if len(warnings) > 0 {
		logger.Info("warnings", "warnings", warnings)
	}

	return result, warnings, nil
}