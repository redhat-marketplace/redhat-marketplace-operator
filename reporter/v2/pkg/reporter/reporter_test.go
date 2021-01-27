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

	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/google/uuid"
	"github.com/meirf/gopart"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Reporter", func() {
	var (
		err           error
		sut           *MarketplaceReporter
		config        *marketplacev1alpha1.MarketplaceConfig
		dir, dir2     string
		uploader      Uploader
		generatedFile string
		cfg           *Config

		startStr = "2020-06-19T00:00:00Z"
		endStr   = "2020-07-19T00:00:00Z"
		start, _ = time.Parse(time.RFC3339, startStr)
		end, _   = time.Parse(time.RFC3339, endStr)
	)

	BeforeEach(func() {
		uploader, err = NewRedHatInsightsUploader(&RedHatInsightsUploaderConfig{
			URL:             "https://cloud.redhat.com",
			ClusterID:       "2858312a-ff6a-41ae-b108-3ed7b12111ef",
			OperatorVersion: "1.0.0",
			Token:           "token",
		})
		Expect(err).To(Succeed())

		uploader.(*RedHatInsightsUploader).client.Transport = &stubRoundTripper{
			roundTrip: func(req *http.Request) *http.Response {
				headers := make(http.Header)
				headers.Add("content-type", "text")

				Expect(req.URL.String()).To(Equal(fmt.Sprintf(uploadURL, uploader.(*RedHatInsightsUploader).URL)), "url does not match expected")
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

		dir, err = ioutil.TempDir("", "report")
		dir2, err = ioutil.TempDir("", "targz")

		Expect(err).To(Succeed())

		cfg = &Config{
			OutputDirectory: dir,
		}

		cfg.SetDefaults()

		config = &marketplacev1alpha1.MarketplaceConfig{
			Spec: marketplacev1alpha1.MarketplaceConfigSpec{
				RhmAccountID: "foo",
				ClusterUUID:  "foo-id",
			},
		}
	})

	BeforeSuite(func() {
		generatedFile = GenerateRandomData(start, end)
	})

	Context("with templates", func() {
		const count = 2976
		const fileCount = 7
		BeforeEach(func() {
			meterDefs := []v1beta1.MeterDefinition{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
						UID:       types.UID("a"),
					},
					Spec: v1beta1.MeterDefinitionSpec{
						Group: "{{ .Labels.meter_kind | lower }}.partner.metering.com",
						Kind:  "App",
						ResourceFilters: []v1beta1.ResourceFilter{
							{
								Namespace: &v1beta1.NamespaceFilter{UseOperatorGroup: true},
								OwnerCRD: &v1beta1.OwnerCRDFilter{
									GroupVersionKind: common.GroupVersionKind{
										APIVersion: "apps.partner.metering.com/v1",
										Kind:       "App",
									},
								},
								WorkloadType: v1beta1.WorkloadTypePod,
							},
						},
						Meters: []v1beta1.MeterWorkload{
							{
								Aggregation:  "sum",
								Query:        "rpc_durations_seconds_sum",
								Metric:       "rpc_durations_seconds_sum",
								WorkloadType: v1beta1.WorkloadTypePod,
								Description:  "{{ .Labels.meter_kind | lower }} description",
							},
							{

								Aggregation:  "sum",
								Query:        "my_query",
								Metric:       "rpc_durations_seconds_count",
								WorkloadType: v1beta1.WorkloadTypePod,
								Description:  "{{ .Labels.meter_kind | lower }} description",
							},
						},
					},
				},
			}

			v1api := getTestAPI(mockResponseRoundTripper(generatedFile, meterDefs))

			sut = &MarketplaceReporter{
				api:       v1api,
				Config:    cfg,
				mktconfig: config,
				report: &marketplacev1alpha1.MeterReport{
					Spec: marketplacev1alpha1.MeterReportSpec{
						StartTime: metav1.Time{Time: start},
						EndTime:   metav1.Time{Time: end},
					},
				},
			}
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
			Expect(len(files)).To(Equal(fileCount))
			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())

				if !strings.Contains(file, "metadata") {
					fileBytes, err := ioutil.ReadFile(file)
					Expect(err).To(Succeed(), "file does not exist")
					data := make(map[string]interface{})
					err = json.Unmarshal(fileBytes, &data)
					Expect(err).To(Succeed(), "file data did not parse to json")

					id := func(element interface{}) string {
						return "row"
					}

					Expect(data).To(MatchAllKeys(Keys{
						"report_slice_id": Not(BeEmpty()),
						"metrics": MatchElements(id, AllowDuplicates, Elements{
							"row": MatchAllKeys(Keys{
								"additionalLabels": MatchAllKeys(Keys{
									"namespace":         Equal("metering-example-operator"),
									"pod":               Equal("example-app-pod"),
									"meter_kind":        Or(Equal("App"), Equal("App2"), Equal("App3")),
									"meter_domain":      Equal("apps.partner.metering.com"),
									"meter_version":     Equal("v1"),
									"service":           Equal("example-app-pod"),
									"meter_description": Or(Equal("app description"), Equal("app2 description"), Equal("app3 description")),
								}),
								"domain":              Or(Equal("app.partner.metering.com"), Equal("app2.partner.metering.com"), Equal("app3.partner.metering.com")),
								"interval_start":      HavePrefix("2020-"),
								"interval_end":        HavePrefix("2020-"),
								"metric_id":           BeAssignableToTypeOf(""),
								"report_period_end":   Equal(endStr),
								"kind":                Or(Equal("App"), Equal("App2"), Equal("App3")),
								"namespace":           Equal("metering-example-operator"),
								"report_period_start": Equal(startStr),
								"resource_name":       Equal("example-app-pod"),
								"workload":            Or(Equal("rpc_durations_seconds_sum"), Equal("rpc_durations_seconds_count")),
								"rhmUsageMetrics": Or(MatchAllKeys(Keys{
									"rpc_durations_seconds_count": BeAssignableToTypeOf(""),
								}),
									MatchAllKeys(Keys{
										"rpc_durations_seconds_sum": BeAssignableToTypeOf(""),
									})),
							}),
						}),
					}))
				}
			}

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
	})

	Context("without templates", func() {
		BeforeEach(func() {
			meterDefs := []v1beta1.MeterDefinition{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
						UID:       types.UID("a"),
					},
					Spec: v1beta1.MeterDefinitionSpec{
						Group: "apps.partner.metering.com",
						Kind:  "App",
						ResourceFilters: []v1beta1.ResourceFilter{
							{
								Namespace: &v1beta1.NamespaceFilter{UseOperatorGroup: true},
								OwnerCRD: &v1beta1.OwnerCRDFilter{
									GroupVersionKind: common.GroupVersionKind{
										APIVersion: "apps.partner.metering.com/v1",
										Kind:       "App",
									},
								},
								WorkloadType: v1beta1.WorkloadTypePod,
							},
						},
						Meters: []v1beta1.MeterWorkload{
							{
								Aggregation:  "sum",
								Query:        "rpc_durations_seconds_sum",
								Metric:       "rpc_durations_seconds_sum",
								WorkloadType: v1beta1.WorkloadTypePod,
							},
							{

								Aggregation:  "sum",
								Query:        "my_query",
								Metric:       "rpc_durations_seconds_count",
								WorkloadType: v1beta1.WorkloadTypePod,
							},
						},
					},
				},
			}

			v1api := getTestAPI(mockResponseRoundTripper(generatedFile, meterDefs))

			sut = &MarketplaceReporter{
				api:       v1api,
				Config:    cfg,
				mktconfig: config,
				report: &marketplacev1alpha1.MeterReport{
					Spec: marketplacev1alpha1.MeterReportSpec{
						StartTime: metav1.Time{Time: start},
						EndTime:   metav1.Time{Time: end},
					},
				},
			}
		})

		It("query, build and submit a report", func(done Done) {
			By("collecting metrics")
			results, errs, err := sut.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())
			Expect(len(results)).To(Equal(1488))

			By("writing report")

			files, err := sut.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())
			Expect(len(files)).To(Equal(4))
			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())

				if !strings.Contains(file, "metadata") {
					fileBytes, err := ioutil.ReadFile(file)
					Expect(err).To(Succeed(), "file does not exist")
					data := make(map[string]interface{})
					err = json.Unmarshal(fileBytes, &data)
					Expect(err).To(Succeed(), "file data did not parse to json")

					id := func(element interface{}) string {
						return "row"
					}

					Expect(data).To(MatchAllKeys(Keys{
						"report_slice_id": Not(BeEmpty()),
						"metrics": MatchElements(id, AllowDuplicates, Elements{
							"row": MatchAllKeys(Keys{
								"additionalLabels": MatchAllKeys(Keys{
									"namespace":     Equal("metering-example-operator"),
									"pod":           Equal("example-app-pod"),
									"meter_kind":    Or(Equal("App"), Equal("App2"), Equal("App3")),
									"meter_domain":  Equal("apps.partner.metering.com"),
									"meter_version": Equal("v1"),
									"service":       Equal("example-app-pod"),
								}),
								"domain":              Or(Equal("apps.partner.metering.com")),
								"interval_start":      HavePrefix("2020-"),
								"interval_end":        HavePrefix("2020-"),
								"metric_id":           BeAssignableToTypeOf(""),
								"report_period_end":   Equal(endStr),
								"kind":                Or(Equal("App"), Equal("App2"), Equal("App3")),
								"namespace":           Equal("metering-example-operator"),
								"report_period_start": Equal(startStr),
								"resource_name":       Equal("example-app-pod"),
								"workload":            Or(Equal("rpc_durations_seconds_sum"), Equal("rpc_durations_seconds_count")),
								"rhmUsageMetrics": Or(MatchAllKeys(Keys{
									"rpc_durations_seconds_count": BeAssignableToTypeOf(""),
								}),
									MatchAllKeys(Keys{
										"rpc_durations_seconds_sum": BeAssignableToTypeOf(""),
									})),
							}),
						}),
					}))
				}
			}

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
	})
})

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

func mockResponseRoundTripper(file string, meterdefinitions []v1beta1.MeterDefinition) RoundTripFunc {
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
			"meter_domain":  "apps.partner.metering.com",
			"meter_kind":    kind,
			"meter_version": "v1",
			"namespace":     "metering-example-operator",
			"pod":           "example-app-pod",
			"service":       "example-app-pod",
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
