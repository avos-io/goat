// Code generated by mockery v2.34.2. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	stats "google.golang.org/grpc/stats"
)

// MockHandler is an autogenerated mock type for the Handler type
type MockHandler struct {
	mock.Mock
}

type MockHandler_Expecter struct {
	mock *mock.Mock
}

func (_m *MockHandler) EXPECT() *MockHandler_Expecter {
	return &MockHandler_Expecter{mock: &_m.Mock}
}

// HandleConn provides a mock function with given fields: _a0, _a1
func (_m *MockHandler) HandleConn(_a0 context.Context, _a1 stats.ConnStats) {
	_m.Called(_a0, _a1)
}

// MockHandler_HandleConn_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'HandleConn'
type MockHandler_HandleConn_Call struct {
	*mock.Call
}

// HandleConn is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 stats.ConnStats
func (_e *MockHandler_Expecter) HandleConn(_a0 interface{}, _a1 interface{}) *MockHandler_HandleConn_Call {
	return &MockHandler_HandleConn_Call{Call: _e.mock.On("HandleConn", _a0, _a1)}
}

func (_c *MockHandler_HandleConn_Call) Run(run func(_a0 context.Context, _a1 stats.ConnStats)) *MockHandler_HandleConn_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(stats.ConnStats))
	})
	return _c
}

func (_c *MockHandler_HandleConn_Call) Return() *MockHandler_HandleConn_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockHandler_HandleConn_Call) RunAndReturn(run func(context.Context, stats.ConnStats)) *MockHandler_HandleConn_Call {
	_c.Call.Return(run)
	return _c
}

// HandleRPC provides a mock function with given fields: _a0, _a1
func (_m *MockHandler) HandleRPC(_a0 context.Context, _a1 stats.RPCStats) {
	_m.Called(_a0, _a1)
}

// MockHandler_HandleRPC_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'HandleRPC'
type MockHandler_HandleRPC_Call struct {
	*mock.Call
}

// HandleRPC is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 stats.RPCStats
func (_e *MockHandler_Expecter) HandleRPC(_a0 interface{}, _a1 interface{}) *MockHandler_HandleRPC_Call {
	return &MockHandler_HandleRPC_Call{Call: _e.mock.On("HandleRPC", _a0, _a1)}
}

func (_c *MockHandler_HandleRPC_Call) Run(run func(_a0 context.Context, _a1 stats.RPCStats)) *MockHandler_HandleRPC_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(stats.RPCStats))
	})
	return _c
}

func (_c *MockHandler_HandleRPC_Call) Return() *MockHandler_HandleRPC_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockHandler_HandleRPC_Call) RunAndReturn(run func(context.Context, stats.RPCStats)) *MockHandler_HandleRPC_Call {
	_c.Call.Return(run)
	return _c
}

// TagConn provides a mock function with given fields: _a0, _a1
func (_m *MockHandler) TagConn(_a0 context.Context, _a1 *stats.ConnTagInfo) context.Context {
	ret := _m.Called(_a0, _a1)

	var r0 context.Context
	if rf, ok := ret.Get(0).(func(context.Context, *stats.ConnTagInfo) context.Context); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(context.Context)
		}
	}

	return r0
}

// MockHandler_TagConn_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'TagConn'
type MockHandler_TagConn_Call struct {
	*mock.Call
}

// TagConn is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 *stats.ConnTagInfo
func (_e *MockHandler_Expecter) TagConn(_a0 interface{}, _a1 interface{}) *MockHandler_TagConn_Call {
	return &MockHandler_TagConn_Call{Call: _e.mock.On("TagConn", _a0, _a1)}
}

func (_c *MockHandler_TagConn_Call) Run(run func(_a0 context.Context, _a1 *stats.ConnTagInfo)) *MockHandler_TagConn_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*stats.ConnTagInfo))
	})
	return _c
}

func (_c *MockHandler_TagConn_Call) Return(_a0 context.Context) *MockHandler_TagConn_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockHandler_TagConn_Call) RunAndReturn(run func(context.Context, *stats.ConnTagInfo) context.Context) *MockHandler_TagConn_Call {
	_c.Call.Return(run)
	return _c
}

// TagRPC provides a mock function with given fields: _a0, _a1
func (_m *MockHandler) TagRPC(_a0 context.Context, _a1 *stats.RPCTagInfo) context.Context {
	ret := _m.Called(_a0, _a1)

	var r0 context.Context
	if rf, ok := ret.Get(0).(func(context.Context, *stats.RPCTagInfo) context.Context); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(context.Context)
		}
	}

	return r0
}

// MockHandler_TagRPC_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'TagRPC'
type MockHandler_TagRPC_Call struct {
	*mock.Call
}

// TagRPC is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 *stats.RPCTagInfo
func (_e *MockHandler_Expecter) TagRPC(_a0 interface{}, _a1 interface{}) *MockHandler_TagRPC_Call {
	return &MockHandler_TagRPC_Call{Call: _e.mock.On("TagRPC", _a0, _a1)}
}

func (_c *MockHandler_TagRPC_Call) Run(run func(_a0 context.Context, _a1 *stats.RPCTagInfo)) *MockHandler_TagRPC_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*stats.RPCTagInfo))
	})
	return _c
}

func (_c *MockHandler_TagRPC_Call) Return(_a0 context.Context) *MockHandler_TagRPC_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockHandler_TagRPC_Call) RunAndReturn(run func(context.Context, *stats.RPCTagInfo) context.Context) *MockHandler_TagRPC_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockHandler creates a new instance of MockHandler. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockHandler(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockHandler {
	mock := &MockHandler{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}