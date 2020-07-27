// +build wireinject

package metric_generator

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

func NewServer(
	opts *Options,
) (*Service, error) {
	panic(wire.Build(
		managers.ProvideCachedClientSet,
		getClientOptions,
		NewMeterCollector,
		controller.SchemeDefinitions,
		reconcileutils.CommandRunnerProviderSet,
		ConvertOptions,
		wire.Struct(new(Service), "*"),
		wire.InterfaceValue(new(logr.Logger), log),
		wire.Value(CollectorRefreshRate(time.Minute*5)),
		wire.Value(collectorStop),
	))
}
