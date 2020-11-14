package harness

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PodFailHandler(testHarness *TestHarness) func(message string, callerSkip ...int) {
	return func(message string, callerSkip ...int) {
		if testHarness != nil {
			printDebug(testHarness)
		}
		Fail(message, callerSkip...)
	}
}

func printDebug(testHarness *TestHarness) {
	lists := []runtime.Object{
		&corev1.PodList{},
		&appsv1.DeploymentList{},
		&appsv1.StatefulSetList{},
		&v1alpha1.MeterBaseList{},
		&v1alpha1.RazeeDeploymentList{},
		&v1alpha1.MarketplaceConfigList{},
	}

	filters := []func(runtime.Object) bool{
		func(obj runtime.Object) bool {
			pod, ok := obj.(*corev1.Pod)

			if !ok {
				return false
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.ContainersReady {
					if cond.Status == corev1.ConditionTrue {
						return true
					}
				}
			}
			return false
		},
	}

	for _, list := range lists {
		testHarness.List(context.TODO(), list, client.InNamespace(testHarness.Config.Namespace))
		printList(list, filters)
	}
}

func printList(list runtime.Object, filters []func(runtime.Object) bool) {
	preamble := "\x1b[1mDEBUG %T\x1b[0m"
	if config.DefaultReporterConfig.NoColor {
		preamble = "DEBUG %T"
	}

	extractedList, err := meta.ExtractList(list)

	if err != nil {
		return
	}

printloop:
	for _, item := range extractedList {
		for _, filter := range filters {
			if filter(item) {
				continue printloop
			}
		}

		typePre := fmt.Sprintf(preamble, item)
		access, _ := meta.Accessor(item)

		access.SetManagedFields([]metav1.ManagedFieldsEntry{})
		data, _ := json.MarshalIndent(item, "", "  ")

		fmt.Fprintf(GinkgoWriter,
			"%s: %s/%s debug output: %s\n", typePre, access.GetName(), access.GetNamespace(), string(data))
	}
}
