package runnables

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PodMonitor struct {
	logger logr.Logger
	cc     ClientCommandRunner
	config PodMonitorConfig
}

type PodMonitorConfig struct {
	Namespace string
	RetryTime time.Duration
}

func NewPodMonitor(
	logger logr.Logger,
	cc ClientCommandRunner,
	config PodMonitorConfig,
) *PodMonitor {
	return &PodMonitor{
		logger: logger,
		cc:     cc,
		config: config,
	}
}

func (a *PodMonitor) Start(stop <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	go a.Run(ctx)

	<-stop
	cancel()
	return nil
}

func (a *PodMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(a.config.RetryTime)
	log := a.logger.WithValues("func", "podMonitor")

	for {
		select {
		case <-ticker.C:
			result, _ := a.cc.Do(context.Background(), a.deleteRestartingPods()...)

			if result.Is(Error) {
				log.Error(result, "failed to clean up bad pods")
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (a *PodMonitor) deleteRestartingPods() []ClientAction {
	log := a.logger.WithValues("func", "deleteRestartingPods")
	list := &corev1.PodList{}

	return []ClientAction{
		HandleResult(
			ListAction(list, client.InNamespace(a.config.Namespace)),
			OnContinue(Call(func() (ClientAction, error) {
				actions := []ClientAction{}
				podNames := []string{}

				for _, pod := range list.Items {
					for _, status := range pod.Status.ContainerStatuses {
						if status.Name == "authcheck" && !status.Ready {
							podNames = append(podNames, pod.Name)
							actions = append(actions,
								DeleteAction(&pod),
							)
							break
						}
					}
				}

				if len(actions) == 0 {
					log.Info("nothing to delete")
					return nil, nil
				}

				log.Info("attempting to delete", "len", len(actions), "podNames", strings.Join(podNames, ","))
				return Do(actions...), nil
			})),
		),
	}
}
