package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Reporter", func() {
	var (
		err              error
		sut              *MarketplaceReporter
		config           *marketplacev1alpha1.MarketplaceConfig
		report           *marketplacev1alpha1.MeterReport
		meterDefinitions []*marketplacev1alpha1.MeterDefinition
		dir, dir2        string

		start, _ = time.Parse(time.RFC3339, "2020-04-19T13:00:00Z")
		end, _   = time.Parse(time.RFC3339, "2020-04-19T16:00:00Z")
	)

	BeforeEach(func() {
		v1api := getTestAPI(mockResponseRoundTripper())
		dir, err = ioutil.TempDir("", "report")
		dir2, err = ioutil.TempDir("", "targz")

		Expect(err).To(Succeed())

		sut = &MarketplaceReporter{
			api:             v1api,
			outputDirectory: dir,
		}

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

		meterDefinitions = []*marketplacev1alpha1.MeterDefinition{
			{
				Spec: marketplacev1alpha1.MeterDefinitionSpec{
					MeterDomain:        "apps.partner.metering.com",
					MeterKind:          "App",
					MeterVersion:       "v1",
					ServiceMeterLabels: []string{"rpc_durations_seconds_count", "rpc_durations_seconds_sum"},
				},
			},
		}
	})

	FIt("query and build a report", func(done Done) {
		By("collecting metrics")
		results, err := sut.CollectMetrics(report, meterDefinitions)

		Expect(err).To(Succeed())
		Expect(results).ToNot(BeEmpty())
		Expect(len(results)).To(Equal(2))

		By("writing report")

		files, err := sut.WriteReport(
			uuid.New(),
			config,
			report,
			results)

		Expect(err).To(Succeed())
		Expect(files).ToNot(BeEmpty())
		Expect(len(files)).To(Equal(2))
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
					return element.(map[string]interface{})["interval_start"].(string)
				}
				Expect(data).To(MatchAllKeys(Keys{
					"report_slice_id": Not(BeEmpty()),
					"metrics": MatchElements(id, IgnoreExtras, Elements{
						"2020-04-19T12:00:00-04:00": MatchAllKeys(Keys{
							"additionalLabels": MatchAllKeys(Keys{
								"namespace": Equal("metering-example-operator"),
								"pod":       Equal("example-app-pod"),
								"service":   Equal("example-app-pod"),
							}),
							"domain":              Equal("apps.partner.metering.com"),
							"interval_end":        Equal("2020-04-19T13:00:00-04:00"),
							"interval_start":      Equal("2020-04-19T12:00:00-04:00"),
							"kind":                Equal("App"),
							"report_period_end":   Not(BeEmpty()),
							"report_period_start": Not(BeEmpty()),
							"rhmUsageMetrics": MatchAllKeys(Keys{
								"rpc_durations_seconds_count": Equal("4355360"),
								"rpc_durations_seconds_sum":   Equal("4355360"),
							}),
							"version": Equal("v1"),
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

		clusterID := "2858312a-ff6a-41ae-b108-3ed7b12111ef"

		token, found := os.LookupEnv("AUTH_TOKEN")

		Expect(found).To(BeTrue())

		uploader := &RedHatInsightsUploader{
			Url:             "https://cloud.redhat.com/api/ingress/v1/upload",
			ClusterID:       clusterID,
			OperatorVersion: "1.0.0",
			Token:           token,
			httpVersion:     2,
		}

		Expect(uploader.UploadFile(fileName)).To(Succeed())

		close(done)
	}, 20)
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

func mockResponseRoundTripper() RoundTripFunc {
	return func(req *http.Request) *http.Response {
		headers := make(http.Header)
		headers.Add("content-type", "application/json")

		Expect(req.URL.String()).To(Equal("http://localhost:9090/api/v1/query_range"), "url does not match expected")

		fileBytes, err := ioutil.ReadFile("../../test/mockresponses/prometheus-query-range.json")

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
