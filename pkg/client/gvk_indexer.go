package client

import (
	"context"
	"fmt"
	"strings"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/logger"
)

const OwnerRefContains = "metadata.ownerReferences.contains"

var log = logger.NewLogger("client")

func AddGVKIndexer(fieldIndexer client.FieldIndexer) error {
	err := fieldIndexer.IndexField(context.TODO(), &corev1.Pod{}, OwnerRefContains, indexGVK)
	if err != nil {
		return err
	}
	err = fieldIndexer.IndexField(context.TODO(), &appsv1.Deployment{}, OwnerRefContains, indexGVK)
	if err != nil {
		return err
	}
	err = fieldIndexer.IndexField(context.TODO(), &appsv1.ReplicaSet{}, OwnerRefContains, indexGVK)
	if err != nil {
		return err
	}
	err = fieldIndexer.IndexField(context.TODO(), &appsv1.DaemonSet{}, OwnerRefContains, indexGVK)
	if err != nil {
		return err
	}
	err = fieldIndexer.IndexField(context.TODO(), &appsv1.StatefulSet{}, OwnerRefContains, indexGVK)
	if err != nil {
		return err
	}
	err = fieldIndexer.IndexField(context.TODO(), &batchv1.Job{}, OwnerRefContains, indexGVK)
	if err != nil {
		return err
	}
	err = fieldIndexer.IndexField(context.TODO(), &monitoringv1.ServiceMonitor{}, OwnerRefContains, indexGVK)
	if err != nil {
		return err
	}
	err = fieldIndexer.IndexField(context.TODO(), &corev1.Service{}, OwnerRefContains, indexGVK)
	if err != nil {
		return err
	}

	return nil
}

func objRefToStr(apiversion, kind string) string {
	result := strings.Split(apiversion, "/")

	if len(result) != 2 {
		return ""
	}

	group, version := result[0], result[1]
	return strings.ToLower(fmt.Sprintf("%s.%s.%s", kind, version, group))
}

func indexGVK(obj runtime.Object) []string {
	results := []string{}
	if meta, ok := obj.(metav1.Object); ok {
		owner := metav1.GetControllerOf(meta)
		if owner == nil {
			return nil
		}

		gvk := objRefToStr(owner.APIVersion, owner.Kind)

		if gvk == "" {
			return nil
		}

		data := []string{gvk, fmt.Sprintf("%s/%s", owner.Name, gvk), string(owner.UID)}
		log.V(4).Info("indexing gvk", "gvk", gvk, "name", meta.GetName(), "namespace", meta.GetNamespace(), "data", data)

		return data
	}

	return results
}

func getOwnersReferences(object metav1.Object, isController bool) []metav1.OwnerReference {
	if object == nil {
		return nil
	}
	// If not filtered as Controller only, then use all the OwnerReferences
	if !isController {
		return object.GetOwnerReferences()
	}
	// If filtered to a Controller, only take the Controller OwnerReference
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		return []metav1.OwnerReference{*ownerRef}
	}
	// No Controller OwnerReference found
	return nil
}
