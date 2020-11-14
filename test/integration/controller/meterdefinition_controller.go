package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/meirf/gopart"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	// promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("MeterReportController", func() {
	BeforeEach(func() {
		Expect(TestHarness.BeforeAll()).To(Succeed())
	})

	AfterEach(func() {
		Expect(TestHarness.AfterAll()).To(Succeed())
	})

	Context("Meterdefinition reconcile", func() {

		var meterdef *v1alpha1.MeterDefinition

		BeforeEach(func(done Done){
			meterdef = &v1alpha1.MeterDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-meterdef",
					Namespace: Namespace,
				},
				Spec: v1alpha1.MeterDefinitionSpec{
					Group:              "testgroup",
					Kind:               "testkind",
					WorkloadVertexType: v1alpha1.WorkloadVertexOperatorGroup,
					Workloads: []v1alpha1.Workload{
						{
							Name:         "test",
							WorkloadType: v1alpha1.WorkloadTypePod,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": "rhm-metric-state",
								},
							},
							MetricLabels: []v1alpha1.MeterLabelQuery{
								{
									Aggregation: "sum",
									Label:       "test",
									Query:       "kube_pod_info",
								},
							},
						},
					},
				},
			}
			
			It("Should create a meterdef on setup",func(){
				// By("create prometheus operator")
				Eventually(func() bool {
					result, _ := CC.Do(
						context.TODO(),
						CreateAction(meterdef),
					)
					return result.Is(Continue)
				}, timeout, interval).Should(BeTrue())
				
			},120)
			
			loc, _ := time.LoadLocation("UTC")
			start := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour(), time.Now().Minute()-1, 0, 0, loc)
			end := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour(), time.Now().Minute(), 0, 0, loc)
			generatedFile := GenerateRandomData(start, end)
			v1api := getTestAPI(mockResponseRoundTripper(generatedFile))
			utils.PrettyPrint(v1api)
			close(done)
		})

		// AfterEach(func(done Done) {
		// 	K8sClient.Delete(context.TODO(), meterdef)

		// 	Expect(K8sClient.Delete(context.TODO(), meterdef)).Should(Succeed())
		// 	close(done)
		// }, 120)

		// It("Should find a meterdef",func(){
		// 	// By("create prometheus operator")
		// 	foundMdef := v1alpha1.MeterDefinition{}
		// 	Eventually(func() bool {
		// 		result, _ := CC.Do(
		// 			context.TODO(),
		// 			GetAction(types.NamespacedName{Name:  "test-meterdef", Namespace: Namespace}, &foundMdef),
		// 		)
		// 		return result.Is(Continue)
		// 	}, timeout, interval).Should(BeTrue())
			
		// },120)
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