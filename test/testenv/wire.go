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
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/config"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

var TestControllerSet = wire.NewSet(
	ControllerSet,
	ProvideControllerFlagSet,
	SchemeDefinitions,
	config.ProvideConfig,
	reconcileutils.ProvideDefaultCommandRunnerProvider,
	wire.Bind(new(reconcileutils.ClientCommandRunnerProvider), new(*reconcileutils.DefaultCommandRunnerProvider)),
)

func initializeLocalSchemes() (LocalSchemes, error) {
	panic(wire.Build(TestControllerSet))
}

func initializeControllers() (ControllerList, error) {
	panic(wire.Build(TestControllerSet))
}
