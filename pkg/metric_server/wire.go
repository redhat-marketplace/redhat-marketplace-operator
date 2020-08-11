// +build wireinject

package metric_server

import (
	monitoringv1client "github.com/coreos/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/go-logr/logr"
	"github.com/google/wire"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	marketplacev1alpha1client "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/generated/clientset/versioned/typed/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/meter_definition"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

func NewServer(
	opts *Options,
) (*Service, error) {
	panic(wire.Build(
		managers.ProvideCachedClientSet,
		getClientOptions,
		controller.SchemeDefinitions,
		reconcileutils.CommandRunnerProviderSet,
		ConvertOptions,
		wire.Struct(new(Service), "*"),
		wire.InterfaceValue(new(logr.Logger), log),
		provideRegistry,
		meter_definition.NewMeterDefinitionStore,
		meter_definition.NewStatusProcessor,
		marketplacev1alpha1client.NewForConfig,
		monitoringv1client.NewForConfig,
		provideContext,
		rhmclient.NewFindOwnerHelper,
		addIndex,
	))
}
