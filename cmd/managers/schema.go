package managers

import (
	"github.com/google/wire"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
)

type OpsSrcSchemeDefinition SchemeDefinition
type MonitoringSchemeDefinition SchemeDefinition

func ProvideOpsSrcScheme() *OpsSrcSchemeDefinition {
	return &OpsSrcSchemeDefinition{
		AddToScheme: opsrcv1.SchemeBuilder.AddToScheme,
	}
}

func ProvideMonitoringScheme() *MonitoringSchemeDefinition {
	return &MonitoringSchemeDefinition{
		AddToScheme: monitoringv1.SchemeBuilder.AddToScheme,
	}
}

var SchemeDefinitions = wire.NewSet(
	ProvideOpsSrcScheme,
	ProvideMonitoringScheme,
)
