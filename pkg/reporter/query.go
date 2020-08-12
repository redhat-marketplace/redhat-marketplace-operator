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
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
)

type PromQuery struct {
	Type     v1alpha1.WorkloadType
	MeterDef struct {
		Name, Namespace string
	}
	Metric        string
	Query         string
	Start, End    time.Time
	Step          time.Duration
	Time          string
	AggregateFunc string
	AggregateBy   []string
}

func (q *PromQuery) makeLeftSide() string {
	switch q.Type {
	case v1alpha1.WorkloadTypePod:
		return fmt.Sprintf(`avg(meterdef_pod_info{meter_def_name="%v",meter_def_namespace="%v"}) without (pod_uid, instance, container, endpoint, job, service)`, q.MeterDef.Name, q.MeterDef.Namespace)
	case v1alpha1.WorkloadTypeServiceMonitor:
		return fmt.Sprintf(`avg(meterdef_service_info{meter_def_name="%v",meter_def_namespace="%v"}) without (pod_uid, instance, container, endpoint, job, pod)`, q.MeterDef.Name, q.MeterDef.Namespace)
	default:
		return "NOTSUPPORTED"
	}
}

func (q *PromQuery) makeJoin() string {
	switch q.Type {
	case v1alpha1.WorkloadTypePod:
		return "* on(pod,namespace) group_right"
	case v1alpha1.WorkloadTypeServiceMonitor:
		return "* on(service,namespace) group_right"
	default:
		return "NOTSUPPORTED"
	}
}

func (q *PromQuery) makeAggregateBy() string {
	switch q.Type {
	case v1alpha1.WorkloadTypePod:
		return fmt.Sprintf(`%v by (pod,namespace)`, q.AggregateFunc)
	case v1alpha1.WorkloadTypeServiceMonitor:
		return fmt.Sprintf(`%v by (service,namespace)`, q.AggregateFunc)
	default:
		return "NOTSUPPORTED"
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

func (r *MarketplaceReporter) queryRange(query *PromQuery) (model.Value, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	timeRange := v1.Range{
		Start: query.Start,
		End:   query.End,
		Step:  query.Step,
	}

	result, warnings, err := r.api.QueryRange(ctx, query.String(), timeRange)

	if err != nil {
		logger.Error(err, "querying prometheus", "warnings", warnings)
		return nil, warnings, err
	}
	if len(warnings) > 0 {
		logger.Info("warnings", "warnings", warnings)
	}

	return result, warnings, nil
}
