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
	"strings"
	"time"

	"emperror.dev/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
)

// FindOwnerHelper is used by metric-state
// dynamically start an informer using the metadataclient to watch resource types determined by the MeterDefinition ownerCRD lookup
// without using an informer & cache, metric-states use of FindOwnerHelper puts undue load on the api-server and gets rate-limited
// resulting in poor lookup time from repeated get requests

type FindOwnerHelper struct {
	client                   *MetadataClient
	informerFactory          metadatainformer.SharedInformerFactory
	resourceInformerMappings map[meta.RESTMapping]informers.GenericInformer
}

type InformerMappings struct {
}

func NewFindOwnerHelper(
	metadataClient *MetadataClient,
) *FindOwnerHelper {

	resourceInformerMappings := make(map[meta.RESTMapping]informers.GenericInformer)

	informerFactory := metadatainformer.NewFilteredSharedInformerFactory(metadataClient.inClient, time.Minute, corev1.NamespaceAll, nil)
	informerFactory.Start(wait.NeverStop)

	return &FindOwnerHelper{
		client:                   metadataClient,
		informerFactory:          informerFactory,
		resourceInformerMappings: resourceInformerMappings,
	}
}

func (f *FindOwnerHelper) FindOwner(name, namespace string, lookupOwner *metav1.OwnerReference) (ownerRefs []metav1.OwnerReference, err error) {
	apiVersionSplit := strings.Split(lookupOwner.APIVersion, "/")
	var group, version string

	if len(apiVersionSplit) == 1 {
		version = lookupOwner.APIVersion
	} else {
		group = apiVersionSplit[0]
		version = apiVersionSplit[1]
	}

	mapping, err := f.client.restMapper.RESTMapping(schema.GroupKind{Group: group, Kind: lookupOwner.Kind}, version)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get mapping")
	}

	// check if we have an informer for the rest mapping, if not, start one and map it
	informer, ok := f.resourceInformerMappings[*mapping]

	if !ok {
		informer = f.informerFactory.ForResource(mapping.Resource)

		go informer.Informer().Run(wait.NeverStop)

		f.informerFactory.WaitForCacheSync(wait.NeverStop)
		cache.WaitForCacheSync(wait.NeverStop, informer.Informer().HasSynced)

		f.resourceInformerMappings[*mapping] = informer
	}

	result, err := informer.Lister().ByNamespace(namespace).Get(name)

	if err != nil {
		// Check if resource is cluster scoped
		result, err = informer.Lister().Get(name)

		if err != nil {
			return nil, errors.WrapWithDetails(err,
				"failed to get resource",
				"key", namespace+"/"+name,
				"kind", lookupOwner.Kind,
				"version", version,
				"group", group,
			)
		}
	}

	o, err := meta.Accessor(result)
	if err != nil {
		return
	}

	return o.GetOwnerReferences(), nil
}
