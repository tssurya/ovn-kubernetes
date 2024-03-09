// Code generated by mockery v2.40.0. DO NOT EDIT.

package mocks

import (
	addressset "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/ovn/address_set"
	mock "github.com/stretchr/testify/mock"

	ops "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/libovsdb/ops"

	ovsdb "github.com/ovn-org/libovsdb/ovsdb"

	sets "k8s.io/apimachinery/pkg/util/sets"
)

// AddressSetFactory is an autogenerated mock type for the AddressSetFactory type
type AddressSetFactory struct {
	mock.Mock
}

// DestroyAddressSet provides a mock function with given fields: dbIDs
func (_m *AddressSetFactory) DestroyAddressSet(dbIDs *ops.DbObjectIDs) error {
	ret := _m.Called(dbIDs)

	if len(ret) == 0 {
		panic("no return value specified for DestroyAddressSet")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*ops.DbObjectIDs) error); ok {
		r0 = rf(dbIDs)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// EnsureAddressSet provides a mock function with given fields: dbIDs
func (_m *AddressSetFactory) EnsureAddressSet(dbIDs *ops.DbObjectIDs) (addressset.AddressSet, error) {
	ret := _m.Called(dbIDs)

	if len(ret) == 0 {
		panic("no return value specified for EnsureAddressSet")
	}

	var r0 addressset.AddressSet
	var r1 error
	if rf, ok := ret.Get(0).(func(*ops.DbObjectIDs) (addressset.AddressSet, error)); ok {
		return rf(dbIDs)
	}
	if rf, ok := ret.Get(0).(func(*ops.DbObjectIDs) addressset.AddressSet); ok {
		r0 = rf(dbIDs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(addressset.AddressSet)
		}
	}

	if rf, ok := ret.Get(1).(func(*ops.DbObjectIDs) error); ok {
		r1 = rf(dbIDs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAddressSet provides a mock function with given fields: dbIDs
func (_m *AddressSetFactory) GetAddressSet(dbIDs *ops.DbObjectIDs) (addressset.AddressSet, error) {
	ret := _m.Called(dbIDs)

	if len(ret) == 0 {
		panic("no return value specified for GetAddressSet")
	}

	var r0 addressset.AddressSet
	var r1 error
	if rf, ok := ret.Get(0).(func(*ops.DbObjectIDs) (addressset.AddressSet, error)); ok {
		return rf(dbIDs)
	}
	if rf, ok := ret.Get(0).(func(*ops.DbObjectIDs) addressset.AddressSet); ok {
		r0 = rf(dbIDs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(addressset.AddressSet)
		}
	}

	if rf, ok := ret.Get(1).(func(*ops.DbObjectIDs) error); ok {
		r1 = rf(dbIDs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewAddressSet provides a mock function with given fields: dbIDs, ips
func (_m *AddressSetFactory) NewAddressSet(dbIDs *ops.DbObjectIDs, ips sets.Set[string]) (addressset.AddressSet, error) {
	ret := _m.Called(dbIDs, ips)

	if len(ret) == 0 {
		panic("no return value specified for NewAddressSet")
	}

	var r0 addressset.AddressSet
	var r1 error
	if rf, ok := ret.Get(0).(func(*ops.DbObjectIDs, sets.Set[string]) (addressset.AddressSet, error)); ok {
		return rf(dbIDs, ips)
	}
	if rf, ok := ret.Get(0).(func(*ops.DbObjectIDs, sets.Set[string]) addressset.AddressSet); ok {
		r0 = rf(dbIDs, ips)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(addressset.AddressSet)
		}
	}

	if rf, ok := ret.Get(1).(func(*ops.DbObjectIDs, sets.Set[string]) error); ok {
		r1 = rf(dbIDs, ips)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewAddressSetOps provides a mock function with given fields: dbIDs, ips
func (_m *AddressSetFactory) NewAddressSetOps(dbIDs *ops.DbObjectIDs, ips sets.Set[string]) (addressset.AddressSet, []ovsdb.Operation, error) {
	ret := _m.Called(dbIDs, ips)

	if len(ret) == 0 {
		panic("no return value specified for NewAddressSetOps")
	}

	var r0 addressset.AddressSet
	var r1 []ovsdb.Operation
	var r2 error
	if rf, ok := ret.Get(0).(func(*ops.DbObjectIDs, sets.Set[string]) (addressset.AddressSet, []ovsdb.Operation, error)); ok {
		return rf(dbIDs, ips)
	}
	if rf, ok := ret.Get(0).(func(*ops.DbObjectIDs, sets.Set[string]) addressset.AddressSet); ok {
		r0 = rf(dbIDs, ips)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(addressset.AddressSet)
		}
	}

	if rf, ok := ret.Get(1).(func(*ops.DbObjectIDs, sets.Set[string]) []ovsdb.Operation); ok {
		r1 = rf(dbIDs, ips)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]ovsdb.Operation)
		}
	}

	if rf, ok := ret.Get(2).(func(*ops.DbObjectIDs, sets.Set[string]) error); ok {
		r2 = rf(dbIDs, ips)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ProcessEachAddressSet provides a mock function with given fields: ownerController, dbIDsType, iteratorFn
func (_m *AddressSetFactory) ProcessEachAddressSet(ownerController string, dbIDsType *ops.ObjectIDsType, iteratorFn addressset.AddressSetIterFunc) error {
	ret := _m.Called(ownerController, dbIDsType, iteratorFn)

	if len(ret) == 0 {
		panic("no return value specified for ProcessEachAddressSet")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, *ops.ObjectIDsType, addressset.AddressSetIterFunc) error); ok {
		r0 = rf(ownerController, dbIDsType, iteratorFn)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewAddressSetFactory creates a new instance of AddressSetFactory. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAddressSetFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *AddressSetFactory {
	mock := &AddressSetFactory{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
