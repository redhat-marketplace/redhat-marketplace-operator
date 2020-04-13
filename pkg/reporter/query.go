package reporter

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Reporter struct {
	api v1.API
	log logr.Logger
}

type ReporterConfig struct {
	Address string
}

func NewReporter(config *ReporterConfig) (*Reporter, error) {
	client, err := api.NewClient(api.Config{
		Address: config.Address,
	})

	if err != nil {
		return nil, err
	}

	v1api := v1.NewAPI(client)
	return &Reporter{
		api: v1api,
		log: logf.Log.WithName("reporter"),
	}, nil
}

func (r *Reporter) queryRange(label string) (model.Value, error) {
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
