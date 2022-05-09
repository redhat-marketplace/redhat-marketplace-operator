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

package engine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/stores"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func CreatePVCListWatch(kubeClient clientset.Interface) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return cache.NewListWatchFromClient(kubeClient.CoreV1().RESTClient(), "persistentvolumeclaims", ns, fields.Everything())
	}
}

func CreatePodListWatch(kubeClient clientset.Interface) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return cache.NewListWatchFromClient(kubeClient.CoreV1().RESTClient(), "pods", ns, fields.Everything())
	}
}

func CreateServiceMonitorListWatch(c *monitoringv1client.MonitoringV1Client) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return cache.NewListWatchFromClient(c.RESTClient(), "servicemonitors", ns, fields.Everything())
	}
}

func CreateServiceListWatch(kubeClient clientset.Interface) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return cache.NewListWatchFromClient(kubeClient.CoreV1().RESTClient(), "services", ns, fields.Everything())
	}
}

func CreateMeterDefinitionV1Beta1Watch(c *marketplacev1beta1client.MarketplaceV1beta1Client) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return cache.NewListWatchFromClient(c.RESTClient(), "meterdefinitions", ns, fields.Everything())
	}
}

func ProvideNamespacedCacheListers(
	ns *filter.NamespaceWatcher,
	log logr.Logger,
	listWatchers ListWatchers,
) *NamespacedCachedListers {
	return &NamespacedCachedListers{
		watcher:     ns,
		log:         log.WithName("namespaced-cached-lister"),
		listers:     map[string][]RunAndStop{},
		makeListers: listWatchers,
	}
}

type NamespacedCachedListers struct {
	watcher *filter.NamespaceWatcher
	log     logr.Logger

	listers     map[string][]RunAndStop
	makeListers ListWatchers
}

func (w *NamespacedCachedListers) Start(ctx context.Context) error {
	namespaceChange := make(chan interface{}, 1)
	timer := time.NewTicker(time.Minute)
	err := w.watcher.RegisterWatch(namespaceChange)
	if err != nil {
		return err
	}

	go func() {
		for range namespaceChange {
			err := retry.OnError(retry.DefaultBackoff, func(_ error) bool {
				return true
			}, func() error {
				return w.namespaceChange(ctx)
			})
			if err != nil {
				w.log.Error(err, "error starting lister")
			}
		}
	}()

	go func() {
		defer timer.Stop()
		defer close(namespaceChange)
		namespaceChange <- true

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				namespaceChange <- true
			}
		}
	}()

	return nil
}

func (w *NamespacedCachedListers) namespaceChange(ctx context.Context) error {
	namespaces := w.watcher.Get()

	for _, ns := range namespaces {
		_, ok := w.listers[ns]

		if !ok {
			w.listers[ns] = []RunAndStop{}

			for _, makeLister := range w.makeListers {
				lister := makeLister(ns)
				err := lister.Start(ctx)
				if err != nil {
					return err
				}
				w.listers[ns] = append(w.listers[ns], lister)
			}
		}
	}

	toDelete := []string{}
	for currentNs, listers := range w.listers {
		found := false
		for _, ns := range namespaces {
			if ns == currentNs {
				found = true
				break
			}
		}

		if !found {
			for _, lister := range listers {
				lister.Stop()
			}
			toDelete = append(toDelete, currentNs)
		}
	}

	for _, ns := range toDelete {
		delete(w.listers, ns)
	}

	return nil
}

type NamespacedListWatcherFunc func(ns string) RunAndStop

type ListWatchers map[string]NamespacedListWatcherFunc

type MeterDefinitionStoreListWatchers ListWatchers

func ProvideMeterDefinitionStoreListWatchers(
	kubeClient clientset.Interface,
	store *stores.MeterDefinitionStore,
	c *monitoringv1client.MonitoringV1Client,
) MeterDefinitionStoreListWatchers {
	return MeterDefinitionStoreListWatchers{
		"pvcs": func(ns string) RunAndStop {
			return providePVCLister(kubeClient, ns, store)
		},
		"pods": func(ns string) RunAndStop {
			return providePodListerRunnable(kubeClient, ns, store)
		},
		"services": func(ns string) RunAndStop {
			return provideServiceListerRunnable(kubeClient, ns, store)
		},
		"servicemonitors": func(ns string) RunAndStop {
			return provideServiceMonitorListerRunnable(c, ns, store)
		},
	}
}

type PVCListerRunnable struct {
	ListerRunnable
}

func providePVCLister(
	kubeClient clientset.Interface,
	ns string,
	store cache.Store,
) *PVCListerRunnable {
	return &PVCListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &corev1.PersistentVolumeClaim{},
				lister:       CreatePVCListWatch(kubeClient),
			},
			Store:     store,
			namespace: ns,
		},
	}
}

type PodListerRunnable struct {
	ListerRunnable
}

func providePodListerRunnable(
	kubeClient clientset.Interface,
	ns string,
	store cache.Store,
) *PodListerRunnable {
	return &PodListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &corev1.Pod{},
				lister:       CreatePodListWatch(kubeClient),
			},
			namespace: ns,
			Store:     store,
		},
	}
}

type ServiceListerRunnable struct {
	ListerRunnable
}

func provideServiceListerRunnable(
	kubeClient clientset.Interface,
	ns string,
	store cache.Store,
) *ServiceListerRunnable {
	return &ServiceListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &corev1.Service{},
				lister:       CreateServiceListWatch(kubeClient),
			},
			namespace: ns,
			Store:     store,
		},
	}
}

type ServiceMonitorListerRunnable struct {
	ListerRunnable
}

func provideServiceMonitorListerRunnable(
	c *monitoringv1client.MonitoringV1Client,
	ns string,
	store cache.Store,
) *ServiceMonitorListerRunnable {
	return &ServiceMonitorListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &monitoringv1.ServiceMonitor{},
				lister:       CreateServiceMonitorListWatch(c),
			},
			namespace: ns,
			Store:     store,
		},
	}
}

type MeterDefinitionListerRunnable struct {
	ListerRunnable
}

func provideMeterDefinitionListerRunnable(
	ns string,
	c *marketplacev1beta1client.MarketplaceV1beta1Client,
	store cache.Store,
) *MeterDefinitionListerRunnable {
	return &MeterDefinitionListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &v1beta1.MeterDefinition{},
				lister:       CreateMeterDefinitionV1Beta1Watch(c),
			},
			namespace: ns,
			Store:     store,
		},
	}
}
