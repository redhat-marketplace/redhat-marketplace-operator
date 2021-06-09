package test

import (
	"embed"
	"io/fs"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

func MustOpen(fs embed.FS, name string) fs.File {
	f, err := fs.Open(name)

	if err != nil {
		panic(err)
	}

	return f
}

type UnstructuredFS struct {
	embed.FS
}

func (u *UnstructuredFS) GetUnstructured(filename string) (*unstructured.Unstructured, error) {
	bts, err := u.ReadFile(filename)

	if err != nil {
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err = dec.Decode(bts, nil, obj)

	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (u *UnstructuredFS) MustGetUnstructured(filename string) *unstructured.Unstructured {
	res, err := u.GetUnstructured(filename)

	if err != nil {
		panic(err)
	}

	return res
}
