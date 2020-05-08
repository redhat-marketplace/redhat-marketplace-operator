package reporter

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var sut *MarketplaceReporter

func TestReporterCollectMetrics(t *testing.T) {
	require.NoError(t, setupAPI(mockResponseRoundTripper(t)), "failed setup")
	start, _ = time.Parse(time.RFC3339, "2020-04-19T13:00:00Z")
	end, _ = time.Parse(time.RFC3339, "2020-04-19T16:00:00Z")

	config := &marketplacev1alpha1.MarketplaceConfig{
		Spec: marketplacev1alpha1.MarketplaceConfigSpec{
			RhmAccountID: "foo",
			ClusterUUID:  "foo-id",
		},
	}

	report := &marketplacev1alpha1.MeterReport{
		Spec: marketplacev1alpha1.MeterReportSpec{
			StartTime: metav1.Time{Time: start},
			EndTime:   metav1.Time{Time: end},
		},
	}

	meterDefinitions := []*marketplacev1alpha1.MeterDefinition{
		{
			Spec: marketplacev1alpha1.MeterDefinitionSpec{
				MeterDomain:        "apps.partner.metering.com",
				MeterKind:          "App",
				ServiceMeterLabels: []string{"rpc_durations_seconds_count", "rpc_durations_seconds_sum"},
			},
		},
	}

	results, err := sut.CollectMetrics(report, meterDefinitions)

	sut.log.Info("results", "results", fmt.Sprintf("%v", results))
	require.NoError(t, err)
	require.NotEmpty(t, results, "map cannot be empty")
	require.Equal(t, 2, len(results))

	files, err := sut.WriteReport(
		uuid.New(),
		config,
		report,
		results)

	require.NoError(t, err)
	require.NotEmpty(t, files, "files slice cannot be empty")
	require.Equal(t, 2, len(files), "file count")
}

// RoundTripFunc .
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func setupAPI(roundTripper RoundTripFunc) error {
	conf := api.Config{
		Address:      "http://localhost:9090",
		RoundTripper: roundTripper,
	}
	client, err := api.NewClient(conf)

	if err != nil {
		return err
	}

	v1api := v1.NewAPI(client)

	logf.SetLogger(logf.ZapLogger(true))
	logger := logf.Log.WithName("reporter")
	sut = &MarketplaceReporter{
		api:   v1api,
		log:   logger,
		debug: logger.V(0),
	}

	return nil
}

func mockResponseRoundTripper(t *testing.T) RoundTripFunc {
	return func(req *http.Request) *http.Response {
		headers := make(http.Header)
		headers.Add("content-type", "application/json")

		assert.Equal(t, req.URL.String(), "http://localhost:9090/api/v1/query_range")
		fileBytes, err := ioutil.ReadFile("../../test/mockresponses/prometheus-query-range.json")
		assert.NoError(t, err, "failed to load response file for mock")
		return &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: ioutil.NopCloser(bytes.NewBuffer(fileBytes)),
			// Must be set to non-nil value or it panics
			Header: headers,
		}
	}
}
