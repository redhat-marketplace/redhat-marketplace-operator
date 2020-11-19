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

package controller

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/meterdefinition"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"

	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type MeterDefinitionController struct {
	*baseDefinition
}

func ProvideMeterDefinitionController(
	commandRunner reconcileutils.ClientCommandRunnerProvider,
	cfg config.OperatorConfig,
) *MeterDefinitionController {
	return &MeterDefinitionController{
		baseDefinition: &baseDefinition{
			AddFunc: func(mgr manager.Manager) error {
				return meterdefinition.Add(mgr, commandRunner, cfg)
			},
			FlagSetFunc: func() *pflag.FlagSet { return nil },
		},
	}
}
