package reporter

import (
	"context"
	"fmt"
	"time"

	"strings"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PromQuery struct {
	Metric     string
	Functions   []string
	Labels     map[string]string
	Start, End time.Time
	Step       time.Duration
	Time       string
	SumBy      []string
}

func (q *PromQuery) String() string {
	labelsArr := make([]string, 0, len(q.Labels))
	for key, val := range q.Labels {
		labelsArr = append(labelsArr, fmt.Sprintf(`%s="%s"`, key, val))
	}

	var sb strings.Builder

	sb.WriteString(q.Metric)

	if len(labelsArr) > 0 {
		sb.WriteString(fmt.Sprintf("{%s}", strings.Join(labelsArr, ",")))

		if q.Time != "" {
			sb.WriteString(fmt.Sprintf("[%s]", q.Time))
		}
	}

	returnString := sb.String()

	if len(q.Functions) > 0 {
		var finalSB strings.Builder
		for i := len(q.Functions) - 1; i >= 0; i-- {
			funcStr := q.Functions[i]

			finalSB.WriteString(fmt.Sprintf("%s(", funcStr))
		}

		finalSB.WriteString(sb.String())

		for range q.Functions {
			finalSB.WriteString(")")
		}

		returnString = finalSB.String()
	}

	if len(q.SumBy) > 0 {
		returnString = fmt.Sprintf("sum by (%s) (%s)", strings.Join(q.SumBy, ","), returnString)
	}

	return returnString
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
		r.log.Error(err, "querying prometheus")
		return nil, warnings, err
	}
	if len(warnings) > 0 {
		r.log.Info("warnings", "warnings", warnings)
	}

	return result, warnings, nil
}
