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

	olmv1alpha1client "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/typed/operators/v1alpha1"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	marketplacev1alpha1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1alpha1"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"

	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func CreatePVCListWatch(kubeClient clientset.Interface) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return kubeClient.CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.CoreV1().PersistentVolumeClaims(ns).Watch(context.TODO(), opts)
			},
		}
	}
}

func CreatePodListWatch(kubeClient clientset.Interface) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return kubeClient.CoreV1().Pods(ns).List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.CoreV1().Pods(ns).Watch(context.TODO(), opts)
			},
		}
	}
}

func CreateServiceMonitorListWatch(c *monitoringv1client.MonitoringV1Client) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return c.ServiceMonitors(ns).List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return c.ServiceMonitors(ns).Watch(context.TODO(), opts)
			},
		}
	}
}

func CreateServiceListWatch(kubeClient clientset.Interface) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return kubeClient.CoreV1().Services(ns).List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.CoreV1().Services(ns).Watch(context.TODO(), opts)
			},
		}
	}
}

func CreateMeterDefinitionV1Beta1Watch(c *marketplacev1beta1client.MarketplaceV1beta1Client) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return c.MeterDefinitions(ns).List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return c.MeterDefinitions(ns).Watch(context.TODO(), opts)
			},
		}
	}
}

func CreateNodeListWatch(kubeClient clientset.Interface) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return kubeClient.CoreV1().Nodes().List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.CoreV1().Nodes().Watch(context.TODO(), opts)
			},
		}
	}
}

func CreateDeploymentListWatch(kubeClient clientset.Interface) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return kubeClient.AppsV1().Deployments(ns).List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.AppsV1().Deployments(ns).Watch(context.TODO(), opts)
			},
		}
	}
}

func CreateClusterServiceVersionListWatch(c *olmv1alpha1client.OperatorsV1alpha1Client) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return c.ClusterServiceVersions(ns).List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return c.ClusterServiceVersions(ns).Watch(context.TODO(), opts)
			},
		}
	}
}

func CreateClusterVersionListWatch(c *configv1client.ConfigV1Client) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return c.ClusterVersions().List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return c.ClusterVersions().Watch(context.TODO(), opts)
			},
		}
	}
}

func CreateConsoleListWatch(c *configv1client.ConfigV1Client) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return c.Consoles().List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return c.Consoles().Watch(context.TODO(), opts)
			},
		}
	}
}

func CreateInfrastructureListWatch(c *configv1client.ConfigV1Client) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return c.Infrastructures().List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return c.Infrastructures().Watch(context.TODO(), opts)
			},
		}
	}
}

func CreateMarketplaceConfigV1Alpha1Watch(c *marketplacev1alpha1client.MarketplaceV1alpha1Client) func(string) cache.ListerWatcher {
	return func(ns string) cache.ListerWatcher {
		return &cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return c.MarketplaceConfigs(ns).List(context.TODO(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return c.MarketplaceConfigs(ns).Watch(context.TODO(), opts)
			},
		}
	}
}
