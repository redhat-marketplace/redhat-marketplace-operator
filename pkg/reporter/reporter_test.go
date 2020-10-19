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
	"bytes"
	"path/filepath"
	"strings"

	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/meirf/gopart"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Reporter", func() {
	const count = 48
	var (
		err           error
		sut           *MarketplaceReporter
		config        *marketplacev1alpha1.MarketplaceConfig
		report        *marketplacev1alpha1.MeterReport
		dir, dir2     string
		uploader      *RedHatInsightsUploader
		generatedFile string

		startStr = "2020-07-18T00:00:00Z"
		endStr   = "2020-07-19T00:00:00Z"
		start, _ = time.Parse(time.RFC3339, startStr)
		end, _   = time.Parse(time.RFC3339, endStr)
	)

	id := func(element interface{}) string {
		return "row"
	}

	additionalLabels := Keys{
		"namespace":             Equal("metering-example-operator"),
		"pod":                   Equal("example-app-pod"),
		"service":               Equal("example-app-pod"),
		"persistentvolumeclaim": Equal("example-pvc"),
	}

	expectedMetricRow := Keys{
		"additionalLabels":    MatchAllKeys(additionalLabels),
		"domain":              Equal("apps.partner.metering.com"),
		"interval_start":      HavePrefix("2020-"),
		"interval_end":        HavePrefix("2020-"),
		"kind":                Or(Equal("App"), Equal("App2")),
		"metric_id":           BeAssignableToTypeOf(""),
		"report_period_end":   Equal(endStr),
		"report_period_start": Equal(startStr),
		"rhmUsageMetrics": MatchAllKeys(Keys{
			"rpc_durations_seconds_count": BeAssignableToTypeOf(""),
			"rpc_durations_seconds_sum":   BeAssignableToTypeOf(""),
		}),
		"version": Equal(""),
	}

	expectedFields := Keys{
		"report_slice_id": Not(BeEmpty()),
		"metrics": MatchElements(id, AllowDuplicates, Elements{
			"row": MatchAllKeys(expectedMetricRow),
		}),
	}

	BeforeEach(func() {
		v1api := getTestAPI(mockResponseRoundTripper(generatedFile))
		dir, err = ioutil.TempDir("", "report")
		dir2, err = ioutil.TempDir("", "targz")

		Expect(err).To(Succeed())

		cfg := &Config{
			OutputDirectory: dir,
		}

		cfg.SetDefaults()

		config = &marketplacev1alpha1.MarketplaceConfig{
			Spec: marketplacev1alpha1.MarketplaceConfigSpec{
				RhmAccountID: "foo",
				ClusterUUID:  "foo-id",
			},
		}

		report = &marketplacev1alpha1.MeterReport{
			Spec: marketplacev1alpha1.MeterReportSpec{
				StartTime: metav1.Time{Time: start},
				EndTime:   metav1.Time{Time: end},
			},
		}

		sut = &MarketplaceReporter{
			api:       v1api,
			Config:    cfg,
			report:    report,
			mktconfig: config,
			meterDefinitions: []marketplacev1alpha1.MeterDefinition{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-meterdef",
						Namespace: "namespace",
					},
					Spec: marketplacev1alpha1.MeterDefinitionSpec{
						Group: "apps.partner.metering.com",
						Kind:  "App",
						Workloads: []marketplacev1alpha1.Workload{
							{
								WorkloadType: "Pod",
								MetricLabels: []marketplacev1alpha1.MeterLabelQuery{
									{
										Label: "rpc_durations_seconds_count",
										Query: "rate(rpc_durations_seconds_count{}[5m])*100",
										// AdditionalFields: []string{"test_field_1", "test_field_2"},
									},
									{
										Label: "rpc_durations_seconds_sum",
										Query: "rate(rpc_durations_seconds_sum{}[5m])*100",
										// AdditionalFields: []string{"test_field_1", "test_field_2"},
									},
								},
							},
						},
					},
				},
			},
		}

		uploader, err = NewRedHatInsightsUploader(&RedHatInsightsUploaderConfig{
			URL:             "https://cloud.redhat.com",
			ClusterID:       "2858312a-ff6a-41ae-b108-3ed7b12111ef",
			OperatorVersion: "1.0.0",
			Token:           "token",
		})

		Expect(err).To(Succeed())
		uploader.client.Transport = &stubRoundTripper{
			roundTrip: func(req *http.Request) *http.Response {
				headers := make(http.Header)
				headers.Add("content-type", "text")

				Expect(req.URL.String()).To(Equal(fmt.Sprintf(uploadURL, uploader.URL)), "url does not match expected")
				_, meta, _ := req.FormFile("file")
				header := meta.Header

				Expect(header.Get("Content-Type")).To(Equal(mktplaceFileUploadType))
				Expect(header.Get("Content-Disposition")).To(
					HavePrefix(`form-data; name="%s"; filename="%s"`, "file", "test-upload.tar.gz"))
				Expect(req.Header.Get("Content-Type")).To(HavePrefix("multipart/form-data; boundary="))
				Expect(req.Header.Get("User-Agent")).To(HavePrefix("marketplace-operator"))

				return &http.Response{
					StatusCode: 202,
					// Send response to be tested
					Body: ioutil.NopCloser(bytes.NewBuffer([]byte{})),
					// Must be set to non-nil value or it panics
					Header: headers,
				}
			},
		}
	})

	BeforeSuite(func() {
		generatedFile = GenerateRandomData(start, end)
	})

	It("query, build and submit a report", func(done Done) {
		By("collecting metrics")
		results, errs, err := sut.CollectMetrics(context.TODO())

		Expect(err).To(Succeed())
		Expect(errs).To(BeEmpty())
		Expect(results).ToNot(BeEmpty())
		Expect(len(results)).To(Equal(count))

		By("writing report")

		files, err := sut.WriteReport(
			uuid.New(),
			results)

		Expect(err).To(Succeed())
		Expect(files).ToNot(BeEmpty())
		Expect(len(files)).To(Equal(2))

		runTestOnFiles(files, expectedFields)

		dirPath := filepath.Dir(files[0])
		fileName := fmt.Sprintf("%s/test-upload.tar.gz", dir2)

		Expect(fileName).ToNot(BeAnExistingFile())

		By(fmt.Sprintf("targz the file %s", fileName))
		Expect(TargzFolder(dirPath, fileName)).To(Succeed())
		Expect(fileName).To(BeAnExistingFile())

		By("uploading file")

		Expect(uploader.UploadFile(fileName)).To(Succeed())

		close(done)
	}, 20)

	When("AdditionalFields are defined on the meter def", func() {

		It("Should include additional fields defined on the meterdefintion on the meter report's additional fields", func(done Done) {

			additionalFields := Keys{
				"test_field_1": Equal("test-value-1"),
				"test_field_2": Equal("test-value-2"),
			}

			for label, matcher := range additionalFields {
				additionalLabels[label] = matcher
			}

			sut.meterDefinitions[0].Spec.Workloads[0].MetricLabels[0].Query = "rate(rpc_durations_seconds_count{test_field_1=test-value-1,test_field_2=test-value-2}[5m])*100"
			sut.meterDefinitions[0].Spec.Workloads[0].MetricLabels[0].AdditionalFields = []string{"test_field_1", "test_field_2"}

			results, errs, err := sut.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())
			Expect(len(results)).To(Equal(count))

			By("writing report")

			files, err := sut.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())
			Expect(len(files)).To(Equal(2))

			runTestOnFiles(files, expectedFields)
			close(done)
		}, 20)

		It("Should throw an error when the additionalFields defined on the MetricLabel do not match the labels on the meterdef query", func(done Done) {

			// sut.meterDefinitions[0].Spec.Workloads[0].MetricLabels[0].Query = "rate(rpc_durations_seconds_count{test_field_1=test-value-1,test_field_2=test-value-2}[5m])*100"
			// sut.meterDefinitions[0].Spec.Workloads[0].MetricLabels[0].AdditionalFields = []string{"wrong_field_1", "test_field_2"}

			sut.meterDefinitions[0].Spec.Workloads = []marketplacev1alpha1.Workload{
				{
					WorkloadType: "Pod",
					MetricLabels: []marketplacev1alpha1.MeterLabelQuery{

						{
							Label:            "rpc_durations_seconds_sum",
							Query:            "rate(rpc_durations_seconds_count{test_field_1=test-value-1,test_field_2=test-value-2}[5m])*100",
							AdditionalFields: []string{"wrong_field_1", "test_field_2"},
						},
					},
				},
			}

			// utils.PrettyPrint(sut.meterDefinitions, "")
			_, errs, _ := sut.CollectMetrics(context.TODO())
			fmt.Println("errs (from test code blah)", errs)
			fmt.Println("errs length (from test code)", len(errs))
			Expect(errs[0]).To(MatchError("Query doesn't contain a key value for additionalField: wrong_field_1"))
			Expect(errs).To(HaveLen(1))
			close(done)
		}, 20)
	})

})

// RoundTripFunc is a type that represents a round trip function call for std http lib
type RoundTripFunc func(req *http.Request) *http.Response

//
func runTestOnFiles(files []string, expectedFields map[interface{}]types.GomegaMatcher) {
	for _, file := range files {
		By(fmt.Sprintf("testing file %s", file))
		Expect(file).To(BeAnExistingFile())

		if !strings.Contains(file, "metadata") {
			fileBytes, err := ioutil.ReadFile(file)
			Expect(err).To(Succeed(), "file does not exist")
			data := make(map[string]interface{})
			err = json.Unmarshal(fileBytes, &data)
			Expect(err).To(Succeed(), "file data did not parse to json")
			Expect(data).To(MatchAllKeys(expectedFields))
		}
	}
}

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

func mockResponseRoundTripper(file string) RoundTripFunc {
	return func(req *http.Request) *http.Response {
		headers := make(http.Header)
		headers.Add("content-type", "application/json")

		Expect(req.URL.String()).To(Equal("http://localhost:9090/api/v1/query_range"), "url does not match expected")

		fileBytes, err := ioutil.ReadFile(file)

		Expect(err).To(Succeed(), "failed to load mock file for response")

		return &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: ioutil.NopCloser(bytes.NewBuffer(fileBytes)),
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
	Metric map[string]string
	Values []interface{}
}

type fakeData struct {
	ResultType string
	Result     []*fakeResult
}

type fakeMetrics struct {
	Status string
	Data   fakeData
}

func GenerateRandomData(start, end time.Time) string {
	next := start
	kinds := []string{"App", "App2"}

	data := make(map[string][]interface{})

	for _, kind := range kinds {
		data[kind] = []interface{}{}
	}

	for next.Before(end) || next.Equal(end) {
		for i := 0; i < 24; i++ {
			rowTime := next.Add(time.Hour * time.Duration(i))

			for _, kind := range kinds {
				num := rand.Float64() * 10
				data[kind] = append(data[kind], []interface{}{rowTime.Unix(), fmt.Sprintf("%v", num)})
			}
		}

		next = next.Add(24 * time.Hour)
	}

	file, err := ioutil.TempFile("", "testfilemetrics")
	Expect(err).To(Succeed(), "failed to parse json")

	makeData := func(kind string) map[string]string {
		return map[string]string{
			"meter_domain":          "apps.partner.metering.com",
			"meter_kind":            kind,
			"namespace":             "metering-example-operator",
			"pod":                   "example-app-pod",
			"service":               "example-app-pod",
			"persistentvolumeclaim": "example-pvc",
			"test_field_1":          "test-value-1",
			"test_field_2":          "test-value-2",
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

	err = ioutil.WriteFile(
		file.Name(),
		marshallBytes,
		0600)
	Expect(err).To(Succeed(), "failed to parse json")

	return file.Name()
}
