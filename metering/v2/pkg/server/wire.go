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

// +build wireinject

package server

import (
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/engine"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/processors"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"time"
)

func NewServer(
	opts *Options,
) (*Service, error) {
	panic(wire.Build(
		engine.NewEngine,
		ProvideNamespaces,
		provideScheme,
		config.GetConfig,
		getClientOptions,
		wire.Struct(new(Service), "*"),
		metrics.ProvidePrometheusData,
		wire.Value(log),
		provideRegistry,
		provideContext,
		wire.Value(processors.StatusFlushDuration(time.Minute)),
	))
}
