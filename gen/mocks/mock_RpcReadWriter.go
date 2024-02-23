// Code generated by mockery v2.34.2. DO NOT EDIT.

package mocks

import (
	context "context"

	goatorepo "github.com/avos-io/goat/gen/goatorepo"
	mock "github.com/stretchr/testify/mock"
)

// MockRpcReadWriter is an autogenerated mock type for the RpcReadWriter type
type MockRpcReadWriter struct {
	mock.Mock
}

type MockRpcReadWriter_Expecter struct {
	mock *mock.Mock
}

func (_m *MockRpcReadWriter) EXPECT() *MockRpcReadWriter_Expecter {
	return &MockRpcReadWriter_Expecter{mock: &_m.Mock}
}

// Read provides a mock function with given fields: _a0
func (_m *MockRpcReadWriter) Read(_a0 context.Context) (*goatorepo.Rpc, error) {
	ret := _m.Called(_a0)

	var r0 *goatorepo.Rpc
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*goatorepo.Rpc, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *goatorepo.Rpc); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*goatorepo.Rpc)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRpcReadWriter_Read_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Read'
type MockRpcReadWriter_Read_Call struct {
	*mock.Call
}

// Read is a helper method to define mock.On call
//   - _a0 context.Context
func (_e *MockRpcReadWriter_Expecter) Read(_a0 interface{}) *MockRpcReadWriter_Read_Call {
	return &MockRpcReadWriter_Read_Call{Call: _e.mock.On("Read", _a0)}
}

func (_c *MockRpcReadWriter_Read_Call) Run(run func(_a0 context.Context)) *MockRpcReadWriter_Read_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockRpcReadWriter_Read_Call) Return(_a0 *goatorepo.Rpc, _a1 error) *MockRpcReadWriter_Read_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRpcReadWriter_Read_Call) RunAndReturn(run func(context.Context) (*goatorepo.Rpc, error)) *MockRpcReadWriter_Read_Call {
	_c.Call.Return(run)
	return _c
}

// Write provides a mock function with given fields: _a0, _a1
func (_m *MockRpcReadWriter) Write(_a0 context.Context, _a1 *goatorepo.Rpc) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *goatorepo.Rpc) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockRpcReadWriter_Write_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Write'
type MockRpcReadWriter_Write_Call struct {
	*mock.Call
}

// Write is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 *goatorepo.Rpc
func (_e *MockRpcReadWriter_Expecter) Write(_a0 interface{}, _a1 interface{}) *MockRpcReadWriter_Write_Call {
	return &MockRpcReadWriter_Write_Call{Call: _e.mock.On("Write", _a0, _a1)}
}

func (_c *MockRpcReadWriter_Write_Call) Run(run func(_a0 context.Context, _a1 *goatorepo.Rpc)) *MockRpcReadWriter_Write_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*goatorepo.Rpc))
	})
	return _c
}

func (_c *MockRpcReadWriter_Write_Call) Return(_a0 error) *MockRpcReadWriter_Write_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockRpcReadWriter_Write_Call) RunAndReturn(run func(context.Context, *goatorepo.Rpc) error) *MockRpcReadWriter_Write_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockRpcReadWriter creates a new instance of MockRpcReadWriter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockRpcReadWriter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRpcReadWriter {
	mock := &MockRpcReadWriter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}