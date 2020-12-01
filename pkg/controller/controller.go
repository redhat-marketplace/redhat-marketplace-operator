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
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type AddController interface {
	Add(mgr manager.Manager) error
	FlagSet() *pflag.FlagSet
}

type baseDefinition struct {
	AddFunc     func(mgr manager.Manager) error
	FlagSetFunc func() *pflag.FlagSet
	Options     controllerOptions
}

func (c *baseDefinition) Add(mgr manager.Manager) error {
	return c.AddFunc(mgr)
}

func (c *baseDefinition) FlagSet() *pflag.FlagSet {
	return c.FlagSetFunc()
}

//go:generate go-options -option ControllerOption -imports=github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils controllerOptions
type controllerOptions struct {
	ClientCommandProvider reconcileutils.ClientCommandRunnerProvider
}

type SetClientCommandRunner interface {
	SetClientCommandRunner(mgr manager.Manager, ccprovider reconcileutils.ClientCommandRunnerProvider) error
}

type ControllerList []AddController

var ControllerSet = wire.NewSet(
	ProvideMarketplaceController,
	ProvideMeterbaseController,
	ProvideRazeeDeployController,
	ProvideMeterDefinitionController,
	ProvideOlmSubscriptionController,
	ProvideMeterReportController,
	ProvideControllerList,
	ProvideNodeController,
	ProvideOlmClusterServiceVersionController,
	ProvideRemoteResourceS3Controller,
	ProvideClusterRegistrationController,
)

func ProvideControllerList(
	myController *MarketplaceController,
	meterbaseC *MeterbaseController,
	meterDefinitionC *MeterDefinitionController,
	razeeC *RazeeDeployController,
	olmSubscriptionC *OlmSubscriptionController,
	meterReport *MeterReportController,
	olmClusterServiceVersionC *OlmClusterServiceVersionController,
	remoteResourceS3C *RemoteResourceS3Controller,
	nodeC *NodeController,
	clusterregistrationC *ClusterRegistrationController,
) ControllerList {
	return []AddController{
		myController,
		meterbaseC,
		meterDefinitionC,
		razeeC,
		olmSubscriptionC,
		meterReport,
		olmClusterServiceVersionC,
		remoteResourceS3C,
		nodeC,
		clusterregistrationC,
	}
}
