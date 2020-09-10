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
	"strings"

	"emperror.dev/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type FindOwnerHelper struct {
	client DynamicClient
}

func NewFindOwnerHelper(
	dynamicClient DynamicClient,
) *FindOwnerHelper {
	return &FindOwnerHelper{
		client: dynamicClient,
	}
}

func (f *FindOwnerHelper) FindOwner(name, namespace string, lookupOwner *metav1.OwnerReference) (owner *metav1.OwnerReference, err error) {
	apiVersionSplit := strings.Split(lookupOwner.APIVersion, "/")
	var group, version string

	if len(apiVersionSplit) == 1 {
		version = lookupOwner.APIVersion
	} else {
		group = apiVersionSplit[0]
		version = apiVersionSplit[1]
	}

	resourceClient, err := f.client.ClientForKind(schema.GroupKind{
		Group: group,
		Kind:  lookupOwner.Kind,
	}, version)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get mapping")
	}

	result, err := resourceClient.Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		return nil, errors.Wrap(err, "failed to get resource")
	}

	o, err := meta.Accessor(result)
	if err != nil {
		return
	}

	owner = metav1.GetControllerOf(o)
	return owner, nil
}
