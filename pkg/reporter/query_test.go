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
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	start, _                = time.Parse(time.RFC3339, "2020-04-19T13:00:00Z")
	end, _                  = time.Parse(time.RFC3339, "2020-04-19T16:00:00Z")
	rpcDurationSecondsQuery = &PromQuery{
		Metric: "rpc_durations_seconds_count",
		Labels: map[string]string{
			"meter_kind":   "App",
			"meter_domain": "apps.partner.metering.com",
		},
		Start: start,
		End:   end,
		Step:  time.Minute * 60,
	}
)

func TestQueryRange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	require.NoError(t, setupAPI(mockResponseRoundTripper(t)), "could not setup api")

	result, warnings, err := sut.queryRange(rpcDurationSecondsQuery)

	require.NoError(t, err)
	require.Empty(t, warnings, "warnings should be empty")
	require.Equal(t, model.ValMatrix, result.Type(), "value type matrix expected")

	matrixResult, ok := result.(model.Matrix)

	require.True(t, ok, "result is not a matrix")
	assert.Equal(t, 1, len(matrixResult), "length does not match")
}

func TestQueryBuilder(t *testing.T) {
	q1 := &PromQuery{
		Metric: "foo",
	}

	assert.Equal(t, "foo", q1.String(), "query with no args")

	queryResult := rpcDurationSecondsQuery.String()
	assert.Contains(t, queryResult, `rpc_durations_seconds_count`)
	assert.Contains(t, queryResult, `meter_kind="App"`)
	assert.Contains(t, queryResult, `meter_domain="apps.partner.metering.com"`)
}
