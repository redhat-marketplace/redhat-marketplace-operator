package prometheus

import (
	"context"
	"sync"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
)

type PrometheusAPIBuilder struct {
	Cfg *config.OperatorConfig
	CC  reconcileutils.ClientCommandRunner

	prometheusAPI *PrometheusAPI
	sync.Mutex
}

func (p *PrometheusAPIBuilder) Build(
	userWorkloadMonitoringEnabled bool,
) (*PrometheusAPI, error) {
	p.Lock()
	defer p.Unlock()

	if p.prometheusAPI != nil {
		return p.prometheusAPI, nil
	}

	var err error

	p.prometheusAPI, err = ProvidePrometheusAPI(context.TODO(),
		p.CC,
		p.Cfg.ControllerValues.DeploymentNamespace,
		userWorkloadMonitoringEnabled)

	if err != nil {
		return nil, err
	}

	return nil, nil
}
