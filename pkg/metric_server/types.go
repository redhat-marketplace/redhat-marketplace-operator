package metric_server

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/kube-state-metrics/pkg/options"
)

var (
	DefaultNamespaces = options.NamespaceList{metav1.NamespaceAll}

	DefaultResources = map[string]struct{}{
		"pods": struct{}{},
		"services": struct{}{},
	}

	DefaultEnabledResources = []string{"pods", "services"}
)

type promLogger struct{}

func (pl promLogger) Println(v ...interface{}) {
	klog.Error(v...)
}
