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

package mock_query

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"io/ioutil"
	"net/http"
	"net/url"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	v1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
)

type StubRoundTripper struct {
	StubRoundTrip RoundTripFunc
}

func (s *StubRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return s.StubRoundTrip(req), nil
}

type FakeResult struct {
	Metric map[string]string
	Values []interface{}
}

type FakeData struct {
	ResultType string
	Result     []*FakeResult
}

type FakeMetrics struct {
	Status string
	Data   FakeData
}

// RoundTripFunc is a type that represents a round trip function call for std http lib
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip is a wrapper function that calls an external function for mocking
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func GetTestAPI(trip RoundTripFunc) v1.API {
	conf := api.Config{
		Address:      "http://localhost:9090",
		RoundTripper: trip,
	}
	client, err := api.NewClient(conf)

	Expect(err).To(Succeed())

	v1api := v1.NewAPI(client)
	return v1api
}
func GenerateMeterInfoResponse(meterdefinitions []v1beta1.MeterDefinition) []byte {
	results := []map[string]interface{}{}
	for _, mdef := range meterdefinitions {
		labels := mdef.ToPrometheusLabels()

		for _, mylabels := range labels {
			fmt.Printf("%+v\n", mylabels)
			labelMap, err := mylabels.ToLabels()
			if err != nil {
				fmt.Printf("%v\n", err)
				panic(err)
			}
			fmt.Printf("%+v\n", labelMap)
			results = append(results, map[string]interface{}{
				"metric": labelMap,
				"values": [][]interface{}{
					{1, "1"},
					{2, "1"},
				},
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

	fmt.Println(string(bytes))

	return bytes
}

func MockResponseRoundTripper(file string, meterdefinitions []v1beta1.MeterDefinition) RoundTripFunc {
	return func(req *http.Request) *http.Response {
		headers := make(http.Header)
		headers.Add("content-type", "application/json")

		Expect(req.URL.String()).To(Equal("http://localhost:9090/api/v1/query_range"), "url does not match expected")

		fileBytes, err := ioutil.ReadFile(file)

		Expect(err).To(Succeed(), "failed to load mock file for response")
		defer req.Body.Close()
		body, err := ioutil.ReadAll(req.Body)

		Expect(err).To(Succeed())

		query, _ := url.ParseQuery(string(body))

		if strings.Contains(query["query"][0], "meterdef_metric_label_info{}") {
			fmt.Println("using meter_label_info")
			meterDefInfo := GenerateMeterInfoResponse(meterdefinitions)
			return &http.Response{
				StatusCode: 200,
				// Send response to be tested
				Body: ioutil.NopCloser(bytes.NewBuffer(meterDefInfo)),
				// Must be set to non-nil value or it panics
				Header: headers,
			}
		}

		return &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: ioutil.NopCloser(bytes.NewBuffer(fileBytes)),
			// Must be set to non-nil value or it panics
			Header: headers,
		}
	}
}
