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

package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	. "github.com/onsi/gomega"

	"github.com/meirf/gopart"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
)

// RoundTripFunc is a type that represents a round trip function call for std http lib
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip is a wrapper function that calls an external function for mocking
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func getTestAPI(trip RoundTripFunc) v1.API {
	conf := api.Config{
		Address:      "http://localhost:9090",
		RoundTripper: trip,
	}
	client, err := api.NewClient(conf)

	Expect(err).To(Succeed())

	v1api := v1.NewAPI(client)
	return v1api
}

func mockResponseRoundTripper(files map[string]string, meterdefinitions []v1beta1.MeterDefinition, start, end time.Time) RoundTripFunc {
	return func(req *http.Request) *http.Response {
		headers := make(http.Header)
		headers.Add("content-type", "application/json")

		Expect(req.URL.String()).To(Equal("http://localhost:9090/api/v1/query_range"), "url does not match expected")

		defer req.Body.Close()
		body, err := io.ReadAll(req.Body)
		Expect(err).To(Succeed())
		query, _ := url.ParseQuery(string(body))

		if strings.Contains(query["query"][0], "meterdef_metric_label_info{}") {
			meterDefInfo := GenerateMeterInfoResponse(meterdefinitions, start, end)
			return &http.Response{
				StatusCode: 200,
				// Send response to be tested
				Body: io.NopCloser(bytes.NewBuffer(meterDefInfo)),
				// Must be set to non-nil value or it panics
				Header: headers,
			}
		}

		var fileBytes []byte
		Expect(err).To(Succeed(), "failed to load mock file for response")

	finddata:
		for _, meterdef := range meterdefinitions {
			for _, meter := range meterdef.Spec.Meters {
				if strings.Contains(query["query"][0], meter.Query) {
					fileBytes, err = os.ReadFile(files[meter.Query])
					Expect(err).To(Succeed())
					break finddata
				}
			}
		}

		Expect(fileBytes).To(Not(HaveLen(0)))

		return &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewBuffer(fileBytes)),
			// Must be set to non-nil value or it panics
			Header: headers,
		}
	}
}

type stubRoundTripper struct {
	roundTrip RoundTripFunc
}

func (s *stubRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return s.roundTrip(req), nil
}

type fakeResult struct {
	Metric map[string]string `json:"metric"`
	Values []interface{}     `json:"values"`
}

type fakeData struct {
	ResultType string
	Result     []*fakeResult
}

type fakeMetrics struct {
	Status string
	Data   fakeData
}

func GenerateSeries(start, end time.Time) []int64 {
	series := []int64{}
	for start := start; start.Before(end); start = start.Add(time.Hour) {
		series = append(series, start.Unix())
	}
	return series
}

func GenerateMeterInfoResponse(meterdefinitions []v1beta1.MeterDefinition, start, end time.Time) []byte {
	results := []map[string]interface{}{}
	series := GenerateSeries(start, end)

	for _, mdef := range meterdefinitions {
		labels := mdef.ToPrometheusLabels()

		for _, mylabels := range labels {
			labelMap, err := mylabels.ToLabels()
			if err != nil {
				panic(err)
			}

			values := make([][]interface{}, len(series), len(series))

			for i, ms := range series {
				values[i] = []interface{}{ms, "1"}
			}

			results = append(results, map[string]interface{}{
				"metric": labelMap,
				"values": values,
			})
		}
	}

	data := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "matrix",
			"result":     results,
		},
	}

	bytes, _ := json.Marshal(&data)

	return bytes
}

func GenerateRandomData(start, end time.Time, queries []string) string {
	next := start
	kinds := queries

	data := make(map[string][]interface{})

	for _, kind := range kinds {
		data[kind] = []interface{}{}
	}

	count := 0

	for next.Before(end) || next.Equal(end) {
		for i := 0; i < 23; i++ {
			rowTime := next.Add(time.Hour * time.Duration(i))

			for _, kind := range kinds {
				count = count + 1
				num := float64(count)
				data[kind] = append(data[kind], []interface{}{rowTime.Unix(), fmt.Sprintf("%v", num)})
			}
		}

		next = next.Add(24 * time.Hour)
	}

	file, err := os.CreateTemp("", "testfilemetrics")
	Expect(err).To(Succeed(), "failed to parse json")

	makeData := func(query string) map[string]string {
		return map[string]string{
			"meter_domain":  "apps.partner.metering.com",
			"meter_query":   query,
			"meter_version": "v1",
			"namespace":     "metering-example-operator",
			"pod":           "example-app-pod",
			"service":       "example-app-service",
		}
	}

	results := []*fakeResult{}

	for _, kind := range kinds {
		for idxRange := range gopart.Partition(len(data[kind]), 24) {
			array := data[kind][idxRange.Low:idxRange.High]
			results = append(results, &fakeResult{
				Metric: makeData(kind),
				Values: array,
			})
		}
	}

	fakem := &fakeMetrics{
		Status: "success",
		Data: fakeData{
			ResultType: "matrix",
			Result:     results,
		},
	}

	marshallBytes, err := json.Marshal(fakem)
	Expect(err).To(Succeed(), "failed to parse json")

	err = os.WriteFile(
		file.Name(),
		marshallBytes,
		0600)
	Expect(err).To(Succeed(), "failed to parse json")

	return file.Name()
}
