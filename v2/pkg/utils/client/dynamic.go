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
	"emperror.dev/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type DynamicClient struct {
	inClient   dynamic.Interface
	restMapper meta.RESTMapper
}

func NewDynamicClient(
	inClient dynamic.Interface,
	restMapper meta.RESTMapper,
) *DynamicClient {
	return &DynamicClient{
		inClient:   inClient,
		restMapper: restMapper,
	}
}

func (c *DynamicClient) ClientForKind(
	gk schema.GroupKind,
	version string) (dynamic.NamespaceableResourceInterface, error) {
	mapping, err := c.restMapper.RESTMapping(gk, version)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get mapping")
	}

	return c.inClient.Resource(mapping.Resource), nil
}
