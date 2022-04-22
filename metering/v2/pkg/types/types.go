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

package types

import (
	"fmt"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type MeterDefinitionEnhancedObject struct {
	metav1.Object

	MeterDefinitions []*v1beta1.MeterDefinition
}

var _ metav1.Object = &MeterDefinitionEnhancedObject{}

type Namespaces []string

func GVKNamespaceKeyFunc(scheme *runtime.Scheme) func(obj interface{}) (string, error) {
	return func(obj interface{}) (string, error) {
		if key, ok := obj.(cache.ExplicitKey); ok {
			return string(key), nil
		}

		v, ok := obj.(client.Object)
		if !ok {
			return "", errors.New("not a runtime object")
		}

		gvk, err := apiutil.GVKForObject(v, scheme)

		if !ok {
			return "", errors.Wrap(err, "can't get gvk")
		}

		apiVersion, kind := gvk.ToAPIVersionAndKind()
		gvkString := apiVersion + "/" + kind + ":"

		meta, err := meta.Accessor(obj)
		if err != nil {
			return "", fmt.Errorf("object has no meta: %v", err)
		}

		if len(meta.GetNamespace()) > 0 {
			return gvkString + meta.GetNamespace() + "/" + meta.GetName(), nil
		}

		return gvkString + meta.GetName(), nil
	}
}
