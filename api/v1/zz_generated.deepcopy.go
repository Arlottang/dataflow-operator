//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2022.

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

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DataflowEngine) DeepCopyInto(out *DataflowEngine) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataflowEngine.
func (in *DataflowEngine) DeepCopy() *DataflowEngine {
	if in == nil {
		return nil
	}
	out := new(DataflowEngine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DataflowEngine) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DataflowEngineList) DeepCopyInto(out *DataflowEngineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DataflowEngine, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataflowEngineList.
func (in *DataflowEngineList) DeepCopy() *DataflowEngineList {
	if in == nil {
		return nil
	}
	out := new(DataflowEngineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DataflowEngineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DataflowEngineSpec) DeepCopyInto(out *DataflowEngineSpec) {
	*out = *in
	out.FrameStandalone = in.FrameStandalone
	in.UserStandalone.DeepCopyInto(&out.UserStandalone)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataflowEngineSpec.
func (in *DataflowEngineSpec) DeepCopy() *DataflowEngineSpec {
	if in == nil {
		return nil
	}
	out := new(DataflowEngineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DataflowEngineStatus) DeepCopyInto(out *DataflowEngineStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataflowEngineStatus.
func (in *DataflowEngineStatus) DeepCopy() *DataflowEngineStatus {
	if in == nil {
		return nil
	}
	out := new(DataflowEngineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FrameStandaloneSpec) DeepCopyInto(out *FrameStandaloneSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FrameStandaloneSpec.
func (in *FrameStandaloneSpec) DeepCopy() *FrameStandaloneSpec {
	if in == nil {
		return nil
	}
	out := new(FrameStandaloneSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UserStandaloneSpec) DeepCopyInto(out *UserStandaloneSpec) {
	*out = *in
	if in.Size != nil {
		in, out := &in.Size, &out.Size
		*out = new(int32)
		**out = **in
	}
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Ports != nil {
		in, out := &in.Ports, &out.Ports
		*out = make([]int32, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UserStandaloneSpec.
func (in *UserStandaloneSpec) DeepCopy() *UserStandaloneSpec {
	if in == nil {
		return nil
	}
	out := new(UserStandaloneSpec)
	in.DeepCopyInto(out)
	return out
}
