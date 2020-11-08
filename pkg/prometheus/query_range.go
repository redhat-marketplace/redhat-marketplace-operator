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
	"strings"
	"time"

	"emperror.dev/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	// v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

var logger = logf.Log.WithName("reporter")

type PromQuery struct {
	Type          v1alpha1.WorkloadType
	MeterDef      types.NamespacedName
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

func QueryRange(query *PromQuery, promApi v1.API) (model.Value, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	timeRange := v1.Range{
		Start: query.Start,
		End:   query.End,
		Step:  query.Step,
	}

	result, warnings, err := promApi.QueryRange(ctx, query.String(), timeRange)
	fmt.Println("RESULT", result)

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
