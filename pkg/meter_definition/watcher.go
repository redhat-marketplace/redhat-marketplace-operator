package meter_definition

import (
	"context"

	monitoringv1client "github.com/coreos/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	marketplacev1alpha1client "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/generated/clientset/versioned/typed/marketplace/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func CreatePVCListWatch(kubeClient clientset.Interface, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return kubeClient.CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return kubeClient.CoreV1().PersistentVolumeClaims(ns).Watch(context.TODO(), opts)
		},
	}
}

func CreatePodListWatch(kubeClient clientset.Interface, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return kubeClient.CoreV1().Pods(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return kubeClient.CoreV1().Pods(ns).Watch(context.TODO(), opts)
		},
	}
}

func CreateServiceMonitorListWatch(c *monitoringv1client.MonitoringV1Client, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return c.ServiceMonitors(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return c.ServiceMonitors(ns).Watch(context.TODO(), opts)
		},
	}
}

func CreateServiceListWatch(kubeClient clientset.Interface, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return kubeClient.CoreV1().Services(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return kubeClient.CoreV1().Services(ns).Watch(context.TODO(), opts)
		},
	}
}

func CreateMeterDefinitionWatch(c *marketplacev1alpha1client.MarketplaceV1alpha1Client, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return c.MeterDefinitions(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return c.MeterDefinitions(ns).Watch(context.TODO(), opts)
		},
	}
}
