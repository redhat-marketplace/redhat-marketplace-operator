// Code generated by mockery v2.20.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	dynamic "k8s.io/client-go/dynamic"

	types "k8s.io/apimachinery/pkg/types"

	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	watch "k8s.io/apimachinery/pkg/watch"
)

// NamespaceableResourceInterface is an autogenerated mock type for the NamespaceableResourceInterface type
type NamespaceableResourceInterface struct {
	mock.Mock
}

// Apply provides a mock function with given fields: ctx, name, obj, options, subresources
func (_m *NamespaceableResourceInterface) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	_va := make([]interface{}, len(subresources))
	for _i := range subresources {
		_va[_i] = subresources[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, name, obj, options)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *unstructured.Unstructured
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *unstructured.Unstructured, v1.ApplyOptions, ...string) (*unstructured.Unstructured, error)); ok {
		return rf(ctx, name, obj, options, subresources...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, *unstructured.Unstructured, v1.ApplyOptions, ...string) *unstructured.Unstructured); ok {
		r0 = rf(ctx, name, obj, options, subresources...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unstructured.Unstructured)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, *unstructured.Unstructured, v1.ApplyOptions, ...string) error); ok {
		r1 = rf(ctx, name, obj, options, subresources...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ApplyStatus provides a mock function with given fields: ctx, name, obj, options
func (_m *NamespaceableResourceInterface) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions) (*unstructured.Unstructured, error) {
	ret := _m.Called(ctx, name, obj, options)

	var r0 *unstructured.Unstructured
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *unstructured.Unstructured, v1.ApplyOptions) (*unstructured.Unstructured, error)); ok {
		return rf(ctx, name, obj, options)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, *unstructured.Unstructured, v1.ApplyOptions) *unstructured.Unstructured); ok {
		r0 = rf(ctx, name, obj, options)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unstructured.Unstructured)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, *unstructured.Unstructured, v1.ApplyOptions) error); ok {
		r1 = rf(ctx, name, obj, options)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Create provides a mock function with given fields: ctx, obj, options, subresources
func (_m *NamespaceableResourceInterface) Create(ctx context.Context, obj *unstructured.Unstructured, options v1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	_va := make([]interface{}, len(subresources))
	for _i := range subresources {
		_va[_i] = subresources[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, obj, options)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *unstructured.Unstructured
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *unstructured.Unstructured, v1.CreateOptions, ...string) (*unstructured.Unstructured, error)); ok {
		return rf(ctx, obj, options, subresources...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *unstructured.Unstructured, v1.CreateOptions, ...string) *unstructured.Unstructured); ok {
		r0 = rf(ctx, obj, options, subresources...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unstructured.Unstructured)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *unstructured.Unstructured, v1.CreateOptions, ...string) error); ok {
		r1 = rf(ctx, obj, options, subresources...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Delete provides a mock function with given fields: ctx, name, options, subresources
func (_m *NamespaceableResourceInterface) Delete(ctx context.Context, name string, options v1.DeleteOptions, subresources ...string) error {
	_va := make([]interface{}, len(subresources))
	for _i := range subresources {
		_va[_i] = subresources[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, name, options)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, v1.DeleteOptions, ...string) error); ok {
		r0 = rf(ctx, name, options, subresources...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteCollection provides a mock function with given fields: ctx, options, listOptions
func (_m *NamespaceableResourceInterface) DeleteCollection(ctx context.Context, options v1.DeleteOptions, listOptions v1.ListOptions) error {
	ret := _m.Called(ctx, options, listOptions)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, v1.DeleteOptions, v1.ListOptions) error); ok {
		r0 = rf(ctx, options, listOptions)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Get provides a mock function with given fields: ctx, name, options, subresources
func (_m *NamespaceableResourceInterface) Get(ctx context.Context, name string, options v1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	_va := make([]interface{}, len(subresources))
	for _i := range subresources {
		_va[_i] = subresources[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, name, options)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *unstructured.Unstructured
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, v1.GetOptions, ...string) (*unstructured.Unstructured, error)); ok {
		return rf(ctx, name, options, subresources...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, v1.GetOptions, ...string) *unstructured.Unstructured); ok {
		r0 = rf(ctx, name, options, subresources...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unstructured.Unstructured)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, v1.GetOptions, ...string) error); ok {
		r1 = rf(ctx, name, options, subresources...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// List provides a mock function with given fields: ctx, opts
func (_m *NamespaceableResourceInterface) List(ctx context.Context, opts v1.ListOptions) (*unstructured.UnstructuredList, error) {
	ret := _m.Called(ctx, opts)

	var r0 *unstructured.UnstructuredList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, v1.ListOptions) (*unstructured.UnstructuredList, error)); ok {
		return rf(ctx, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, v1.ListOptions) *unstructured.UnstructuredList); ok {
		r0 = rf(ctx, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unstructured.UnstructuredList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, v1.ListOptions) error); ok {
		r1 = rf(ctx, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Namespace provides a mock function with given fields: _a0
func (_m *NamespaceableResourceInterface) Namespace(_a0 string) dynamic.ResourceInterface {
	ret := _m.Called(_a0)

	var r0 dynamic.ResourceInterface
	if rf, ok := ret.Get(0).(func(string) dynamic.ResourceInterface); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(dynamic.ResourceInterface)
		}
	}

	return r0
}

// Patch provides a mock function with given fields: ctx, name, pt, data, options, subresources
func (_m *NamespaceableResourceInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options v1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	_va := make([]interface{}, len(subresources))
	for _i := range subresources {
		_va[_i] = subresources[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, name, pt, data, options)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *unstructured.Unstructured
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, types.PatchType, []byte, v1.PatchOptions, ...string) (*unstructured.Unstructured, error)); ok {
		return rf(ctx, name, pt, data, options, subresources...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, types.PatchType, []byte, v1.PatchOptions, ...string) *unstructured.Unstructured); ok {
		r0 = rf(ctx, name, pt, data, options, subresources...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unstructured.Unstructured)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, types.PatchType, []byte, v1.PatchOptions, ...string) error); ok {
		r1 = rf(ctx, name, pt, data, options, subresources...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Update provides a mock function with given fields: ctx, obj, options, subresources
func (_m *NamespaceableResourceInterface) Update(ctx context.Context, obj *unstructured.Unstructured, options v1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	_va := make([]interface{}, len(subresources))
	for _i := range subresources {
		_va[_i] = subresources[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, obj, options)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *unstructured.Unstructured
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *unstructured.Unstructured, v1.UpdateOptions, ...string) (*unstructured.Unstructured, error)); ok {
		return rf(ctx, obj, options, subresources...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *unstructured.Unstructured, v1.UpdateOptions, ...string) *unstructured.Unstructured); ok {
		r0 = rf(ctx, obj, options, subresources...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unstructured.Unstructured)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *unstructured.Unstructured, v1.UpdateOptions, ...string) error); ok {
		r1 = rf(ctx, obj, options, subresources...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateStatus provides a mock function with given fields: ctx, obj, options
func (_m *NamespaceableResourceInterface) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options v1.UpdateOptions) (*unstructured.Unstructured, error) {
	ret := _m.Called(ctx, obj, options)

	var r0 *unstructured.Unstructured
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *unstructured.Unstructured, v1.UpdateOptions) (*unstructured.Unstructured, error)); ok {
		return rf(ctx, obj, options)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *unstructured.Unstructured, v1.UpdateOptions) *unstructured.Unstructured); ok {
		r0 = rf(ctx, obj, options)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unstructured.Unstructured)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *unstructured.Unstructured, v1.UpdateOptions) error); ok {
		r1 = rf(ctx, obj, options)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Watch provides a mock function with given fields: ctx, opts
func (_m *NamespaceableResourceInterface) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	ret := _m.Called(ctx, opts)

	var r0 watch.Interface
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, v1.ListOptions) (watch.Interface, error)); ok {
		return rf(ctx, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, v1.ListOptions) watch.Interface); ok {
		r0 = rf(ctx, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(watch.Interface)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, v1.ListOptions) error); ok {
		r1 = rf(ctx, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewNamespaceableResourceInterface interface {
	mock.TestingT
	Cleanup(func())
}

// NewNamespaceableResourceInterface creates a new instance of NamespaceableResourceInterface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewNamespaceableResourceInterface(t mockConstructorTestingTNewNamespaceableResourceInterface) *NamespaceableResourceInterface {
	mock := &NamespaceableResourceInterface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
