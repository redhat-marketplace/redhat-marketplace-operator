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

package client

import (
	"context"
	"fmt"
	"strings"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	IndexMeterDefinitionPods = "meterdefinition.marketplace.redhat.com/pods"

	IndexOwnerRefContains = ".metadata.ownerReferences"
	IndexAnnotations      = ".metadata.annotations"

	IndexOperatorSourceNamespaces   = "operatorsource.namespace"
	IndexOperatorSourceProvidedAPIs = "operatorsource.providedAPIs"

	IndexUID = ".metadata.UID"
)

var log = logf.Log.WithName("client")

func AddOwningControllerIndex(fieldIndexer client.FieldIndexer, types []runtime.Object) error {
	for _, rType := range types {
		err := fieldIndexer.IndexField(context.Background(), rType, IndexOwnerRefContains, indexOwner)

		if err != nil {
			return err
		}
	}

	return nil
}

func AddUIDIndex(fieldIndexer client.FieldIndexer, types []runtime.Object) error {
	for _, rType := range types {
		err := fieldIndexer.IndexField(
			context.Background(),
			rType,
			IndexUID,
			func(obj runtime.Object) []string {
				if meta, ok := obj.(metav1.Object); ok {
					return []string{string(meta.GetUID())}
				}
				return []string{}
			})
		if err != nil {
			return err
		}
	}
	return nil
}

func AddOperatorSourceIndex(fieldIndexer client.FieldIndexer) error {
	err := fieldIndexer.IndexField(context.Background(),
		&olmv1.OperatorGroup{},
		IndexOperatorSourceNamespaces,
		indexOperatorSourceNamespaces)

	if err != nil {
		return err
	}

	return fieldIndexer.IndexField(
		context.Background(),
		&olmv1.OperatorGroup{},
		IndexOperatorSourceProvidedAPIs,
		indexOperatorSourceProvidedAPIs)
}

func AddAnnotationIndex(fieldIndexer client.FieldIndexer, types []runtime.Object) error {
	for _, rType := range types {
		err := fieldIndexer.IndexField(
			context.Background(),
			rType, IndexAnnotations, indexAnnotations)

		if err != nil {
			return err
		}
	}

	return nil
}

func ObjRefToStr(apiversion, kind string) string {
	result := strings.Split(apiversion, "/")

	if len(result) != 2 {
		return ""
	}

	group, version := result[0], result[1]
	return strings.ToLower(fmt.Sprintf("%s.%s.%s", kind, version, group))
}

func indexAnnotations(obj runtime.Object) []string {
	results := []string{}
	if meta, ok := obj.(metav1.Object); ok {

		for key, val := range meta.GetAnnotations() {
			results = append(results, fmt.Sprintf("%s=%s", key, val))
		}
	}

	return results
}

func indexOwner(obj runtime.Object) []string {
	results := []string{}
	if meta, ok := obj.(metav1.Object); ok {
		for _, ref := range meta.GetOwnerReferences() {
			results = append(results, string(ref.UID))
		}

		log.V(4).Info("indexing owenrs", "name", meta.GetName(), "namespace", meta.GetNamespace(), "results", results)

		return results
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

func indexOperatorSourceNamespaces(obj runtime.Object) []string {
	og, ok := obj.(*olmv1.OperatorGroup)

	if !ok {
		return []string{}
	}

	return og.Status.Namespaces
}

func indexOperatorSourceProvidedAPIs(obj runtime.Object) []string {
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
