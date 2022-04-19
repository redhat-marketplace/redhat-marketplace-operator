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
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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
