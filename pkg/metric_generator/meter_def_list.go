package metric_generator

import (
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MeterDefList is a list of MeterDefinitions
type MeterDefList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// List of pods.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md
	Items []marketplacev1alpha1.MeterDefinition `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (in *MeterDefList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *MeterDefList) DeepCopy() *MeterDefList {
	if in == nil {
		return nil
	}
	out := new(MeterDefList)
	in.DeepCopyInto(out)
	return out
}

func (obj *MeterDefList) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }

func (in *MeterDefList) DeepCopyInto(out *MeterDefList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]marketplacev1alpha1.MeterDefinition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}
