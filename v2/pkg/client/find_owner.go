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
	"sync"
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
	client        *MetadataClient
	informers     *InformerMappings
	accessChecker AccessChecker
	sync.Mutex
}

func NewFindOwnerHelper(
	ctx context.Context,
	metadataClient *MetadataClient,
	accessChecker AccessChecker,
) *FindOwnerHelper {
	return &FindOwnerHelper{
		client:        metadataClient,
		informers:     NewInformerMappings(ctx, metadataClient),
		accessChecker: accessChecker,
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

	_, err = f.accessChecker.CheckAccess(group, version, lookupOwner.Kind, namespace)
	if err != nil {
		return nil, err
	}

	mapping, err := f.client.restMapper.RESTMapping(schema.GroupKind{Group: group, Kind: lookupOwner.Kind}, version)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get mapping")
	}

	informer := f.informers.GetInformer(*mapping)
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

type InformerMappings struct {
	ctx                      context.Context
	cancel                   context.CancelFunc
	resourceInformerMappings map[meta.RESTMapping]informers.GenericInformer
	informerFactory          metadatainformer.SharedInformerFactory

	sync.Mutex
}

func NewInformerMappings(
	ctx context.Context,
	metadataClient *MetadataClient,
) *InformerMappings {
	childCtx, cancel := context.WithCancel(ctx)

	informerFactory := metadatainformer.NewFilteredSharedInformerFactory(
		metadataClient.inClient,
		time.Hour,
		corev1.NamespaceAll,
		nil)
	informerFactory.Start(childCtx.Done())

	return &InformerMappings{
		ctx:                      childCtx,
		cancel:                   cancel,
		informerFactory:          informerFactory,
		resourceInformerMappings: make(map[meta.RESTMapping]informers.GenericInformer),
	}
}

func (i *InformerMappings) GetInformer(mapping meta.RESTMapping) informers.GenericInformer {
	i.Lock()
	defer i.Unlock()

	informer, ok := i.resourceInformerMappings[mapping]
	if !ok {
		informer = i.informerFactory.ForResource(mapping.Resource)
		go informer.Informer().Run(i.ctx.Done())

		i.informerFactory.WaitForCacheSync(wait.NeverStop)
		cache.WaitForCacheSync(wait.NeverStop, informer.Informer().HasSynced)

		informer = &cachedListerInformer{informer: informer.Informer(), lister: informer.Lister()}
		i.resourceInformerMappings[mapping] = informer
	}

	return informer
}

type cachedListerInformer struct {
	informer cache.SharedIndexInformer
	lister   cache.GenericLister
}

func (c *cachedListerInformer) Lister() cache.GenericLister {
	return c.lister
}

func (c *cachedListerInformer) Informer() cache.SharedIndexInformer {
	return c.informer
}
