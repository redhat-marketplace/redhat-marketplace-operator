//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2020 IBM Co..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package common

import (
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Features) DeepCopyInto(out *Features) {
	*out = *in
	if in.Deployment != nil {
		in, out := &in.Deployment, &out.Deployment
		*out = new(bool)
		**out = **in
	}
	if in.Registration != nil {
		in, out := &in.Registration, &out.Registration
		*out = new(bool)
		**out = **in
	}
	if in.EnableMeterDefinitionCatalogServer != nil {
		in, out := &in.EnableMeterDefinitionCatalogServer, &out.EnableMeterDefinitionCatalogServer
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Features.
func (in *Features) DeepCopy() *Features {
	if in == nil {
		return nil
	}
	out := new(Features)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GroupVersionKind) DeepCopyInto(out *GroupVersionKind) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GroupVersionKind.
func (in *GroupVersionKind) DeepCopy() *GroupVersionKind {
	if in == nil {
		return nil
	}
	out := new(GroupVersionKind)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *JobReference) DeepCopyInto(out *JobReference) {
	*out = *in
	if in.StartTime != nil {
		in, out := &in.StartTime, &out.StartTime
		*out = (*in).DeepCopy()
	}
	if in.CompletionTime != nil {
		in, out := &in.CompletionTime, &out.CompletionTime
		*out = (*in).DeepCopy()
	}
	if in.Active != nil {
		in, out := &in.Active, &out.Active
		*out = new(int32)
		**out = **in
	}
	if in.Succeeded != nil {
		in, out := &in.Succeeded, &out.Succeeded
		*out = new(int32)
		**out = **in
	}
	if in.Failed != nil {
		in, out := &in.Failed, &out.Failed
		*out = new(int32)
		**out = **in
	}
	if in.BackoffLimit != nil {
		in, out := &in.BackoffLimit, &out.BackoffLimit
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new JobReference.
func (in *JobReference) DeepCopy() *JobReference {
	if in == nil {
		return nil
	}
	out := new(JobReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MeterDefinitionCatalogServerConfig) DeepCopyInto(out *MeterDefinitionCatalogServerConfig) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MeterDefinitionCatalogServerConfig.
func (in *MeterDefinitionCatalogServerConfig) DeepCopy() *MeterDefinitionCatalogServerConfig {
	if in == nil {
		return nil
	}
	out := new(MeterDefinitionCatalogServerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedNameReference) DeepCopyInto(out *NamespacedNameReference) {
	*out = *in
	if in.GroupVersionKind != nil {
		in, out := &in.GroupVersionKind, &out.GroupVersionKind
		*out = new(GroupVersionKind)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedNameReference.
func (in *NamespacedNameReference) DeepCopy() *NamespacedNameReference {
	if in == nil {
		return nil
	}
	out := new(NamespacedNameReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Result) DeepCopyInto(out *Result) {
	*out = *in
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = make([]ResultValues, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Result.
func (in *Result) DeepCopy() *Result {
	if in == nil {
		return nil
	}
	out := new(Result)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResultValues) DeepCopyInto(out *ResultValues) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResultValues.
func (in *ResultValues) DeepCopy() *ResultValues {
	if in == nil {
		return nil
	}
	out := new(ResultValues)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceReference) DeepCopyInto(out *ServiceReference) {
	*out = *in
	out.TargetPort = in.TargetPort
	if in.TLSConfig != nil {
		in, out := &in.TLSConfig, &out.TLSConfig
		*out = new(v1.TLSConfig)
		(*in).DeepCopyInto(*out)
	}
	in.BearerTokenSecret.DeepCopyInto(&out.BearerTokenSecret)
	if in.BasicAuth != nil {
		in, out := &in.BasicAuth, &out.BasicAuth
		*out = new(v1.TLSConfig)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceReference.
func (in *ServiceReference) DeepCopy() *ServiceReference {
	if in == nil {
		return nil
	}
	out := new(ServiceReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Target) DeepCopyInto(out *Target) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(model.LabelSet, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Target.
func (in *Target) DeepCopy() *Target {
	if in == nil {
		return nil
	}
	out := new(Target)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadResource) DeepCopyInto(out *WorkloadResource) {
	*out = *in
	in.NamespacedNameReference.DeepCopyInto(&out.NamespacedNameReference)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadResource.
func (in *WorkloadResource) DeepCopy() *WorkloadResource {
	if in == nil {
		return nil
	}
	out := new(WorkloadResource)
	in.DeepCopyInto(out)
	return out
}
