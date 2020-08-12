package client

import (
	"context"
	"strings"

	"emperror.dev/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)


type FindOwnerHelper struct {
	client     dynamic.Interface
	restMapper meta.RESTMapper
}

func NewFindOwnerHelper(
	inClient dynamic.Interface,
	restMapper meta.RESTMapper,
) *FindOwnerHelper {
	return &FindOwnerHelper{
		client:     inClient,
		restMapper: restMapper,
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

	mapping, err := f.restMapper.RESTMapping(schema.GroupKind{
		Group: group,
		Kind:  lookupOwner.Kind,
	}, version)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get mapping")
	}

	result, err := f.client.Resource(mapping.Resource).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})

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
