/*
Copyright 2023 IBM Co..
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package manifests

import (
	"fmt"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/controller/operators/olm/overrides/inject"
	corev1 "k8s.io/api/core/v1"
)

// Generic version of github.com/operator-framework/operator-lifecycle-manager/pkg/controller/operators/olm/overrides/inject
// Override a general PodSpec

func injectSubscriptionConfig(podSpec *corev1.PodSpec, subConfig *olmv1alpha1.SubscriptionConfig) error {
	if err := inject.InjectEnvIntoDeployment(podSpec, subConfig.Env); err != nil {
		return fmt.Errorf("failed to inject proxy env variable(s) - %v", err)
	}

	if err := inject.InjectVolumesIntoDeployment(podSpec, subConfig.Volumes); err != nil {
		return fmt.Errorf("failed to inject volume(s) - %v", err)
	}

	if err := inject.InjectVolumeMountsIntoDeployment(podSpec, subConfig.VolumeMounts); err != nil {
		return fmt.Errorf("failed to inject volumeMounts(s) - %v", err)
	}

	if err := inject.InjectTolerationsIntoDeployment(podSpec, subConfig.Tolerations); err != nil {
		return fmt.Errorf("failed to inject toleration(s) - %v", err)
	}

	if err := inject.InjectResourcesIntoDeployment(podSpec, subConfig.Resources); err != nil {
		return fmt.Errorf("failed to inject resources - %v", err)
	}

	if err := inject.InjectNodeSelectorIntoDeployment(podSpec, subConfig.NodeSelector); err != nil {
		return fmt.Errorf("failed to inject nodeSelector - %v", err)
	}

	if err := inject.OverrideDeploymentAffinity(podSpec, subConfig.Affinity); err != nil {
		return fmt.Errorf("failed to inject affinity - %v", err)
	}

	return nil
}
