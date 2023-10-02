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
