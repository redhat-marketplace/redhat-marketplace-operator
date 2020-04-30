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
