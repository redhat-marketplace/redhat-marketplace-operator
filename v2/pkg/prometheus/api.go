// Copyright 2022 IBM Corp.
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

package prometheus

import (
	"context"
	"fmt"
	"sync"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
)

type PrometheusAPIType string

const UserWorkload PrometheusAPIType = "UserWorkload"
const RHMWorkload PrometheusAPIType = "RHMWorkload"

type PrometheusAPIBuilder struct {
	Cfg *config.OperatorConfig
	CC  reconcileutils.ClientCommandRunner

	prometheusAPIs map[PrometheusAPIType]*PrometheusAPI
	mu             sync.Mutex
}

func (p *PrometheusAPIBuilder) Get(apiType PrometheusAPIType) (*PrometheusAPI, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.prometheusAPIs == nil {
		p.prometheusAPIs = make(map[PrometheusAPIType]*PrometheusAPI)
	}

	if promApi, ok := p.prometheusAPIs[apiType]; ok {
		if promApi == nil {
			delete(p.prometheusAPIs, apiType)
			return nil, fmt.Errorf("api is nil %s", string(apiType))
		}

		_, err := promApi.Buildinfo(context.TODO())
		if err != nil {
			delete(p.prometheusAPIs, apiType)
			return nil, err
		}

		return promApi, nil
	}

	err := p.set(apiType)
	if err != nil {
		return nil, err
	}

	api, ok := p.prometheusAPIs[apiType]

	if !ok || api == nil {
		return nil, fmt.Errorf("api type not found %s", string(apiType))
	}

	return api, nil
}

func (p *PrometheusAPIBuilder) set(apiType PrometheusAPIType) error {
	pApi, err := ProvidePrometheusAPI(context.TODO(),
		p.CC,
		p.Cfg.ControllerValues.DeploymentNamespace,
		apiType)

	if err == nil {
		p.prometheusAPIs[apiType] = pApi
	}

	return err
}

func (p *PrometheusAPIBuilder) GetAPITypeFromFlag(userWorkloadMonitoringEnabled bool) PrometheusAPIType {
	if userWorkloadMonitoringEnabled {
		return UserWorkload
	}
	return RHMWorkload
}
