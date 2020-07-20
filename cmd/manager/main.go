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

package main

import (
	"fmt"
	"os"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	loggerf "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/logger"
)

var (
	logger            = loggerf.NewLogger("marketplaceControllerManager")
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383
)

func provideOptions(kscheme *runtime.Scheme) (*manager.Options, error) {
	watchNamespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logger.Error(err, "Failed to get watch namespace")
		return nil, err
	}

	return &manager.Options{
		Namespace:          watchNamespace,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		Scheme:             kscheme,
	}, nil
}

func makeMarketplaceController(
	controllerFlags *controller.ControllerFlagSet,
	controllerList controller.ControllerList,
	mgr manager.Manager,
) *managers.ControllerMain {
	return &managers.ControllerMain{
		Name: "redhat-marketplace-operator",
		FlagSets: []*pflag.FlagSet{
			(*pflag.FlagSet)(controllerFlags),
		},
		Controllers: controllerList,
		Manager:     mgr,
	}
}

func main() {
	marketplaceController, err := InitializeMarketplaceController()

	if err != nil {
		logger.Error(err, "failed to start marketplace controller")
		os.Exit(1)
	}

	marketplaceController.Run()
}
