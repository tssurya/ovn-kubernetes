// Code generated by mockery v2.14.0. DO NOT EDIT.

package mocks

import (
	corev1 "k8s.io/api/core/v1"
	cache "k8s.io/client-go/tools/cache"

	factory "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/factory"

	labels "k8s.io/apimachinery/pkg/labels"

	mock "github.com/stretchr/testify/mock"

	v1 "k8s.io/api/discovery/v1"
)

// NodeWatchFactory is an autogenerated mock type for the NodeWatchFactory type
type NodeWatchFactory struct {
	mock.Mock
}

// AddEndpointSliceHandler provides a mock function with given fields: handlerFuncs, processExisting
func (_m *NodeWatchFactory) AddEndpointSliceHandler(handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{}) error) (*factory.Handler, error) {
	ret := _m.Called(handlerFuncs, processExisting)

	var r0 *factory.Handler
	if rf, ok := ret.Get(0).(func(cache.ResourceEventHandler, func([]interface{}) error) *factory.Handler); ok {
		r0 = rf(handlerFuncs, processExisting)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*factory.Handler)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(cache.ResourceEventHandler, func([]interface{}) error) error); ok {
		r1 = rf(handlerFuncs, processExisting)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// AddFilteredServiceHandler provides a mock function with given fields: namespace, handlerFuncs, processExisting
func (_m *NodeWatchFactory) AddFilteredServiceHandler(namespace string, handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{}) error) (*factory.Handler, error) {
	ret := _m.Called(namespace, handlerFuncs, processExisting)

	var r0 *factory.Handler
	if rf, ok := ret.Get(0).(func(string, cache.ResourceEventHandler, func([]interface{}) error) *factory.Handler); ok {
		r0 = rf(namespace, handlerFuncs, processExisting)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*factory.Handler)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, cache.ResourceEventHandler, func([]interface{}) error) error); ok {
		r1 = rf(namespace, handlerFuncs, processExisting)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// AddNamespaceHandler provides a mock function with given fields: handlerFuncs, processExisting
func (_m *NodeWatchFactory) AddNamespaceHandler(handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{}) error) (*factory.Handler, error) {
	ret := _m.Called(handlerFuncs, processExisting)

	var r0 *factory.Handler
	if rf, ok := ret.Get(0).(func(cache.ResourceEventHandler, func([]interface{}) error) *factory.Handler); ok {
		r0 = rf(handlerFuncs, processExisting)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*factory.Handler)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(cache.ResourceEventHandler, func([]interface{}) error) error); ok {
		r1 = rf(handlerFuncs, processExisting)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// AddPodHandler provides a mock function with given fields: handlerFuncs, processExisting
func (_m *NodeWatchFactory) AddPodHandler(handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{}) error) (*factory.Handler, error) {
	ret := _m.Called(handlerFuncs, processExisting)

	var r0 *factory.Handler
	if rf, ok := ret.Get(0).(func(cache.ResourceEventHandler, func([]interface{}) error) *factory.Handler); ok {
		r0 = rf(handlerFuncs, processExisting)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*factory.Handler)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(cache.ResourceEventHandler, func([]interface{}) error) error); ok {
		r1 = rf(handlerFuncs, processExisting)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// AddServiceHandler provides a mock function with given fields: handlerFuncs, processExisting
func (_m *NodeWatchFactory) AddServiceHandler(handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{}) error) (*factory.Handler, error) {
	ret := _m.Called(handlerFuncs, processExisting)

	var r0 *factory.Handler
	if rf, ok := ret.Get(0).(func(cache.ResourceEventHandler, func([]interface{}) error) *factory.Handler); ok {
		r0 = rf(handlerFuncs, processExisting)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*factory.Handler)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(cache.ResourceEventHandler, func([]interface{}) error) error); ok {
		r1 = rf(handlerFuncs, processExisting)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEndpointSlice provides a mock function with given fields: namespace, name
func (_m *NodeWatchFactory) GetEndpointSlice(namespace string, name string) (*v1.EndpointSlice, error) {
	ret := _m.Called(namespace, name)

	var r0 *v1.EndpointSlice
	if rf, ok := ret.Get(0).(func(string, string) *v1.EndpointSlice); ok {
		r0 = rf(namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.EndpointSlice)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEndpointSlices provides a mock function with given fields: namespace, svcName
func (_m *NodeWatchFactory) GetEndpointSlices(namespace string, svcName string) ([]*v1.EndpointSlice, error) {
	ret := _m.Called(namespace, svcName)

	var r0 []*v1.EndpointSlice
	if rf, ok := ret.Get(0).(func(string, string) []*v1.EndpointSlice); ok {
		r0 = rf(namespace, svcName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.EndpointSlice)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(namespace, svcName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNamespace provides a mock function with given fields: name
func (_m *NodeWatchFactory) GetNamespace(name string) (*corev1.Namespace, error) {
	ret := _m.Called(name)

	var r0 *corev1.Namespace
	if rf, ok := ret.Get(0).(func(string) *corev1.Namespace); ok {
		r0 = rf(name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*corev1.Namespace)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNamespaces provides a mock function with given fields:
func (_m *NodeWatchFactory) GetNamespaces() ([]*corev1.Namespace, error) {
	ret := _m.Called()

	var r0 []*corev1.Namespace
	if rf, ok := ret.Get(0).(func() []*corev1.Namespace); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*corev1.Namespace)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNode provides a mock function with given fields: name
func (_m *NodeWatchFactory) GetNode(name string) (*corev1.Node, error) {
	ret := _m.Called(name)

	var r0 *corev1.Node
	if rf, ok := ret.Get(0).(func(string) *corev1.Node); ok {
		r0 = rf(name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*corev1.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNodes provides a mock function with given fields:
func (_m *NodeWatchFactory) GetNodes() ([]*corev1.Node, error) {
	ret := _m.Called()

	var r0 []*corev1.Node
	if rf, ok := ret.Get(0).(func() []*corev1.Node); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*corev1.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPod provides a mock function with given fields: namespace, name
func (_m *NodeWatchFactory) GetPod(namespace string, name string) (*corev1.Pod, error) {
	ret := _m.Called(namespace, name)

	var r0 *corev1.Pod
	if rf, ok := ret.Get(0).(func(string, string) *corev1.Pod); ok {
		r0 = rf(namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*corev1.Pod)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPods provides a mock function with given fields: namespace
func (_m *NodeWatchFactory) GetPods(namespace string) ([]*corev1.Pod, error) {
	ret := _m.Called(namespace)

	var r0 []*corev1.Pod
	if rf, ok := ret.Get(0).(func(string) []*corev1.Pod); ok {
		r0 = rf(namespace)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*corev1.Pod)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(namespace)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetService provides a mock function with given fields: namespace, name
func (_m *NodeWatchFactory) GetService(namespace string, name string) (*corev1.Service, error) {
	ret := _m.Called(namespace, name)

	var r0 *corev1.Service
	if rf, ok := ret.Get(0).(func(string, string) *corev1.Service); ok {
		r0 = rf(namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*corev1.Service)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListNodes provides a mock function with given fields: selector
func (_m *NodeWatchFactory) ListNodes(selector labels.Selector) ([]*corev1.Node, error) {
	ret := _m.Called(selector)

	var r0 []*corev1.Node
	if rf, ok := ret.Get(0).(func(labels.Selector) []*corev1.Node); ok {
		r0 = rf(selector)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*corev1.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(labels.Selector) error); ok {
		r1 = rf(selector)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LocalPodInformer provides a mock function with given fields:
func (_m *NodeWatchFactory) LocalPodInformer() cache.SharedIndexInformer {
	ret := _m.Called()

	var r0 cache.SharedIndexInformer
	if rf, ok := ret.Get(0).(func() cache.SharedIndexInformer); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cache.SharedIndexInformer)
		}
	}

	return r0
}

// NodeInformer provides a mock function with given fields:
func (_m *NodeWatchFactory) NodeInformer() cache.SharedIndexInformer {
	ret := _m.Called()

	var r0 cache.SharedIndexInformer
	if rf, ok := ret.Get(0).(func() cache.SharedIndexInformer); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cache.SharedIndexInformer)
		}
	}

	return r0
}

// RemoveEndpointSliceHandler provides a mock function with given fields: handler
func (_m *NodeWatchFactory) RemoveEndpointSliceHandler(handler *factory.Handler) {
	_m.Called(handler)
}

// RemoveNamespaceHandler provides a mock function with given fields: handler
func (_m *NodeWatchFactory) RemoveNamespaceHandler(handler *factory.Handler) {
	_m.Called(handler)
}

// RemovePodHandler provides a mock function with given fields: handler
func (_m *NodeWatchFactory) RemovePodHandler(handler *factory.Handler) {
	_m.Called(handler)
}

// RemoveServiceHandler provides a mock function with given fields: handler
func (_m *NodeWatchFactory) RemoveServiceHandler(handler *factory.Handler) {
	_m.Called(handler)
}

// Shutdown provides a mock function with given fields:
func (_m *NodeWatchFactory) Shutdown() {
	_m.Called()
}

// Start provides a mock function with given fields:
func (_m *NodeWatchFactory) Start() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewNodeWatchFactory interface {
	mock.TestingT
	Cleanup(func())
}

// NewNodeWatchFactory creates a new instance of NodeWatchFactory. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewNodeWatchFactory(t mockConstructorTestingTNewNodeWatchFactory) *NodeWatchFactory {
	mock := &NodeWatchFactory{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}