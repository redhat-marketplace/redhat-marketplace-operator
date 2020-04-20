package reporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/require"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var sut *MarketplaceReporter

func setupAPI() error {
	conf := api.Config{
		Address: "http://localhost:9090",
	}
	client, err := api.NewClient(conf)

	if err != nil {
		return err
	}

	v1api := v1.NewAPI(client)

	logf.SetLogger(logf.ZapLogger(true))
	sut = &MarketplaceReporter{
		api: v1api,
		log: logf.Log.WithName("reporter"),
	}

	return nil
}

func TestReporterCollectMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	setupAPI()
	start, _ = time.Parse(time.RFC3339, "2020-04-19T13:00:00Z")
	end, _ = time.Parse(time.RFC3339, "2020-04-19T16:00:00Z")

	report := &marketplacev1alpha1.MeterReport{
		Spec: marketplacev1alpha1.MeterReportSpec{
			StartTime: metav1.Time{start},
			EndTime: metav1.Time{end},
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

	sut.log.Info("results", "results", fmt.Sprintf("%v",results))
	require.NoError(t, err)
	require.NotEmpty(t, results, "map cannot be empty")
	require.Equal(t, 2, len(results))
}
