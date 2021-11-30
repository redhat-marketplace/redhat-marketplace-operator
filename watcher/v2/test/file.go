// Copyright 2021 IBM Corp.
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
