package client

import (
	"context"
	"fmt"
	"strings"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MeterDefinitionGVK  = "meterdefinition.marketplace.redhat.com/gvk"
	MeterDefinitionPods = "meterdefinition.marketplace.redhat.com/pods"

	OwnerRefContains    = "metadata.ownerReferences.contains"

	OperatorSourceNamespaces   = "operatorsource.namespace"
	OperatorSourceProvidedAPIs = "operatorsource.providedAPIs"
)

var log = logger.NewLogger("client")

func AddOperatorSourceIndex(fieldIndexer client.FieldIndexer) error {
	err := fieldIndexer.IndexField(context.Background(), &olmv1.OperatorGroup{}, OperatorSourceNamespaces, IndexOperatorSourceNamespaces)

	if err != nil {
		return err
	}

	return fieldIndexer.IndexField(context.Background(), &olmv1.OperatorGroup{}, OperatorSourceProvidedAPIs, IndexOperatorSourceProvidedAPIs)
}

func AddMeterDefIndex(fieldIndexer client.FieldIndexer) error {
	return fieldIndexer.IndexField(context.Background(), &marketplacev1alpha1.MeterDefinition{}, MeterDefinitionGVK, IndexMeterDefinitionGVK)
}

func ObjRefToStr(apiversion, kind string) string {
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

		gvk := ObjRefToStr(owner.APIVersion, owner.Kind)

		if gvk == "" {
			return nil
		}

		data := []string{
			"gvk:" + gvk,
			"named:" + fmt.Sprintf("%s/%s", owner.Name, gvk),
			"uid:" + string(owner.UID),
		}
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

func IndexMeterDefinitionGVK(obj runtime.Object) []string {
	meterDef, ok := obj.(*marketplacev1alpha1.MeterDefinition)

	if !ok {
		return []string{}
	}

	gvk := ObjRefToStr(meterDef.Spec.Group+"/"+meterDef.Spec.Version, meterDef.Spec.Kind)
	return []string{gvk}
}

func IndexOperatorSourceNamespaces(obj runtime.Object) []string {
	og, ok := obj.(*olmv1.OperatorGroup)

	if !ok {
		return []string{}
	}

	return og.Status.Namespaces
}

func IndexOperatorSourceProvidedAPIs(obj runtime.Object) []string {
	og, ok := obj.(*olmv1.OperatorGroup)

	if !ok {
		return []string{}
	}

	val, ok := og.GetAnnotations()["olm.providedAPIs"]

	if !ok {
		return []string{}
	}

	vals := strings.Split(val, ",")

	return vals
}
