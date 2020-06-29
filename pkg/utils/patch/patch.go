package patch

import (
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
	Calculate(currentObject, modifiedObject runtime.Object, opts ...patch.CalculateOption) (*patch.PatchResult, error)
}
