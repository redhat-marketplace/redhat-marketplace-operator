package managers

import (
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/google/wire"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
)

type OpsSrcSchemeDefinition SchemeDefinition
type MonitoringSchemeDefinition SchemeDefinition

func ProvideOpsSrcScheme() *OpsSrcSchemeDefinition {
	return &OpsSrcSchemeDefinition{
		Name:        "opsrcv1",
		AddToScheme: opsrcv1.SchemeBuilder.AddToScheme,
	}
}

func ProvideMonitoringScheme() *MonitoringSchemeDefinition {
	return &MonitoringSchemeDefinition{
		Name:        "monitoringv1",
		AddToScheme: monitoringv1.AddToScheme,
	}
}

var SchemeDefinitions = wire.NewSet(
	ProvideOpsSrcScheme,
	ProvideMonitoringScheme,
)
