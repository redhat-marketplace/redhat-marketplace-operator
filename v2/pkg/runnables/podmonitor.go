// Copyright 2021 IBM Corp.
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

package runnables

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type PodMonitor struct {
	logger logr.Logger
	cc     ClientCommandRunner
	client kubernetes.Interface
	config PodMonitorConfig
}

type PodMonitorConfig struct {
	Namespace string
	RetryTime time.Duration
}

func NewPodMonitor(
	logger logr.Logger,
	client kubernetes.Interface,
	cc ClientCommandRunner,
	config PodMonitorConfig,
) *PodMonitor {
	return &PodMonitor{
		logger: logger,
		cc:     cc,
		config: config,
		client: client,
	}
}

func (a *PodMonitor) NeedLeaderElection() bool {
	return true
}

func (a *PodMonitor) Start(stop <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go a.Run(ctx)

	select {
	case <-ctx.Done():
		return nil
	case <-stop:
		cancel()
		return nil
	}
}

func (a *PodMonitor) Run(ctx context.Context) error {
	log := a.logger.WithValues("func", "podMonitor")
	log.Info("starting podMonitor")

	podClient := a.client.CoreV1().Pods(a.config.Namespace)
	podWatcher, err := podClient.Watch(ctx, v1.ListOptions{})

	if err != nil {
		log.Error(err, "error on watch")
		return err
	}

	defer podWatcher.Stop()

	for {
		select {
		case evt := <-podWatcher.ResultChan():
			switch evt.Type {
			case watch.Modified:
				pod, ok := evt.Object.(*corev1.Pod)

				if !ok {
					continue
				}

				for _, status := range pod.Status.ContainerStatuses {
					if status.Name == "authcheck" && !status.Ready {
						if pod.Spec.RestartPolicy != corev1.RestartPolicyNever && !(status.RestartCount > 2) {
							break
						}

						pod, err := podClient.Get(context.TODO(), pod.Name, v1.GetOptions{})

						if !k8serrors.IsNotFound(err) {
							podClient.Delete(context.TODO(), pod.Name, v1.DeleteOptions{})
						}

						break
					}
				}
			}
		case <-ctx.Done():
			return nil
		}
		time.Sleep(1*time.Second)
	}
}
