package patch

import (
	"emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
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
	patch.IgnoreVolumeClaimTemplateTypeMetaAndStatus())

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
