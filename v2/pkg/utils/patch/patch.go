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

package patch

import (
	"encoding/json"

	"emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/goph/emperror"
	"k8s.io/apimachinery/pkg/runtime"
)

// PatchAnnotator is the interface for creating new patch annotations
// using the patch library
type PatchAnnotator interface {
	GetOriginalConfiguration(obj runtime.Object) ([]byte, error)
	SetOriginalConfiguration(obj runtime.Object, original []byte) error
	GetModifiedConfiguration(obj runtime.Object, annotate bool) ([]byte, error)
	SetLastAppliedAnnotation(obj runtime.Object) error
}

// PatchMaker is the interface for creating new patches using the patch
// library
type PatchMaker interface {
	Calculate(
		currentObject,
		modifiedObject runtime.Object,
		opts ...patch.CalculateOption) (*patch.PatchResult, error)
}

var (
	IgnoreStatusFields                         = patch.IgnoreStatusFields
	IgnoreVolumeClaimTemplateTypeMetaAndStatus = patch.IgnoreVolumeClaimTemplateTypeMetaAndStatus
)

type Patcher struct {
	PatchAnnotator
	PatchMaker
	PatchOptions []patch.CalculateOption
}

var RHMDefaultPatcher = NewPatcher(
	"marketplace.redhat.com/last-applied",
	patch.IgnoreStatusFields(),
	IgnoreMetadata(),
)

func NewPatcher(
	annotation string,
	options ...patch.CalculateOption,
) Patcher {
	annotator := patch.NewAnnotator(annotation)

	return Patcher{
		PatchAnnotator: annotator,
		PatchMaker:     patch.NewPatchMaker(annotator),
		PatchOptions:   options,
	}
}

func (p Patcher) Calculate(
	currentObject,
	modifiedObject runtime.Object,
) (*patch.PatchResult, error) {
	c, err := p.PatchMaker.Calculate(
		currentObject,
		modifiedObject,
		p.PatchOptions...,
	)

	if err != nil {
		return nil, errors.Wrap(err, "error creating patch")
	}

	return c, err
}

func IgnoreMetadata() patch.CalculateOption {
	return func(current, modified []byte) ([]byte, []byte, error) {
		current, err := deleteMetadata(current)
		if err != nil {
			return []byte{}, []byte{}, emperror.Wrap(err, "could not delete status field from current byte sequence")
		}

		modified, err = deleteMetadata(modified)
		if err != nil {
			return []byte{}, []byte{}, emperror.Wrap(err, "could not delete status field from modified byte sequence")
		}

		return current, modified, nil
	}
}

func deleteMetadata(obj []byte) ([]byte, error) {
	var objectMap map[string]interface{}
	err := json.Unmarshal(obj, &objectMap)
	if err != nil {
		return []byte{}, emperror.Wrap(err, "could not unmarshal byte sequence")
	}

	if o, ok := objectMap["metadata"]; ok {
		o := o.(map[string]interface{})
		if _, ok := o["creationTimestamp"]; ok {
			delete(o, "creationTimestamp")
		}
		if _, ok := o["uid"]; ok {
			delete(o, "uid")
		}
		if _, ok := o["selfLink"]; ok {
			delete(o, "selfLink")
		}
		if _, ok := o["resourceVersion"]; ok {
			delete(o, "resourceVersion")
		}
	}

	obj, err = json.Marshal(objectMap)
	if err != nil {
		return []byte{}, emperror.Wrap(err, "could not marshal byte sequence")
	}

	return obj, nil
}
