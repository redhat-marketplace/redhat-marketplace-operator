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
