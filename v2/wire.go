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

package main

import (
	"github.com/go-logr/logr"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/runnables"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
)

func InitializeMarketplaceController(mgr ctrl.Manager) (*InjectableManager, error) {
	panic(wire.Build(
		builder.ControllerManagedBy,
		wire.InterfaceValue(new(logr.Logger), ctrl.Log),
		config.ProvideConfig,
		reconcileutils.CommandRunnerProviderSet,
		managers.ProvideManagerSet,
		runnables.NewPodMonitor,
		providePodMonitorConfig,
	))
}
