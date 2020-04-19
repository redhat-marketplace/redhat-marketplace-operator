package reporter

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type MarketplaceReporter struct {
	config *MarketplaceReporterConfig
	api v1.API
	log logr.Logger
}

func NewMarketplaceReporter(config *MarketplaceReporterConfig) (*MarketplaceReporter, error) {
	client, err := api.NewClient(config.apiConfig)

	if err != nil {
		return nil, err
	}

	v1api := v1.NewAPI(client)
	return &MarketplaceReporter{
		config: config,
		api: v1api,
		log: logf.Log.WithName("reporter"),
	}, nil
}

func (r *MarketplaceReporter) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Become the leader before proceeding
	err := leader.Become(ctx, r.config.ObjectMeta.GetName())

	if err != nil {
		return err
	}

	return nil
}

func (r *MarketplaceReporter) queryRange(label string) (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	timeRange := v1.Range{
		Start: time.Now().Add(6 * -time.Hour),
		End:   time.Now(),
		Step:  60 * time.Minute,
	}

	result, warnings, err := r.api.QueryRange(ctx, label, timeRange)
	defer cancel()

	if err != nil {
		r.log.Error(err, "querying prometheus")
		return nil, err
	}
	if len(warnings) > 0 {
		r.log.Info("warnings", "warnings", warnings)
	}

	return result, nil
}
