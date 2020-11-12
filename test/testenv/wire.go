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
// The build tag makes sure the stub is not built in the final build.

package testenv

import (
	"context"

	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/reporter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var testControllerSet = wire.NewSet(
	provideQueryPromFunc,
	provideAPIClient,
	ControllerSet,
	ProvideControllerFlagSet,
	SchemeDefinitions,
	managers.ProvideConfiglessManagerSet,
	config.ProvideConfig,
	reconcileutils.ProvideDefaultCommandRunnerProvider,
	provideOptions,
	makeMarketplaceController,
	wire.Bind(new(reconcileutils.ClientCommandRunnerProvider), new(*reconcileutils.DefaultCommandRunnerProvider)),
)

func initializeMainCtrl(cfg *rest.Config) (*managers.ControllerMain, error) {
	panic(wire.Build(testControllerSet))
}

func provideOptions(kscheme *runtime.Scheme) (*manager.Options, error) {
	return &manager.Options{
		Namespace:          "",
		Scheme:             kscheme,
		MetricsBindAddress: "0",
	}, nil
}

func makeMarketplaceController(
	controllerList controller.ControllerList,
	mgr manager.Manager,
) *managers.ControllerMain {
	return &managers.ControllerMain{
		Name:        "redhat-marketplace-operator",
		FlagSets:    []*pflag.FlagSet{},
		Controllers: controllerList,
		Manager:     mgr,
	}
}

func provideContext() context.Context {
	return context.TODO()
}

func provideQueryPromFunc() reconcileutils.QueryForPrometheusServiceFunc {
	return reporter.QueryForPrometheusService
}

func provideAPIClient() reconcileutils.ProvideAPIClientFunc {
	return reporter.ProvideApiClient
}
