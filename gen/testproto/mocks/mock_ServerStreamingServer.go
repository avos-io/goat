// Code generated by mockery v2.45.0. DO NOT EDIT.

package mocks

import (
	context "context"

	metadata "google.golang.org/grpc/metadata"

	mock "github.com/stretchr/testify/mock"

	testproto "github.com/avos-io/goat/gen/testproto"
)

// MockServerStreamingServer is an autogenerated mock type for the ServerStreamingServer type
type MockServerStreamingServer[Res interface{}] struct {
	mock.Mock
}

type MockServerStreamingServer_Expecter[Res interface{}] struct {
	mock *mock.Mock
}

func (_m *MockServerStreamingServer[Res]) EXPECT() *MockServerStreamingServer_Expecter[Res] {
	return &MockServerStreamingServer_Expecter[Res]{mock: &_m.Mock}
}

// Context provides a mock function with given fields:
func (_m *MockServerStreamingServer[Res]) Context() context.Context {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Context")
	}

	var r0 context.Context
	if rf, ok := ret.Get(0).(func() context.Context); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(context.Context)
		}
	}

	return r0
}

// MockServerStreamingServer_Context_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Context'
type MockServerStreamingServer_Context_Call[Res interface{}] struct {
	*mock.Call
}

// Context is a helper method to define mock.On call
func (_e *MockServerStreamingServer_Expecter[Res]) Context() *MockServerStreamingServer_Context_Call[Res] {
	return &MockServerStreamingServer_Context_Call[Res]{Call: _e.mock.On("Context")}
}

func (_c *MockServerStreamingServer_Context_Call[Res]) Run(run func()) *MockServerStreamingServer_Context_Call[Res] {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockServerStreamingServer_Context_Call[Res]) Return(_a0 context.Context) *MockServerStreamingServer_Context_Call[Res] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockServerStreamingServer_Context_Call[Res]) RunAndReturn(run func() context.Context) *MockServerStreamingServer_Context_Call[Res] {
	_c.Call.Return(run)
	return _c
}

// RecvMsg provides a mock function with given fields: m
func (_m *MockServerStreamingServer[Res]) RecvMsg(m interface{}) error {
	ret := _m.Called(m)

	if len(ret) == 0 {
		panic("no return value specified for RecvMsg")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockServerStreamingServer_RecvMsg_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RecvMsg'
type MockServerStreamingServer_RecvMsg_Call[Res interface{}] struct {
	*mock.Call
}

// RecvMsg is a helper method to define mock.On call
//   - m interface{}
func (_e *MockServerStreamingServer_Expecter[Res]) RecvMsg(m interface{}) *MockServerStreamingServer_RecvMsg_Call[Res] {
	return &MockServerStreamingServer_RecvMsg_Call[Res]{Call: _e.mock.On("RecvMsg", m)}
}

func (_c *MockServerStreamingServer_RecvMsg_Call[Res]) Run(run func(m interface{})) *MockServerStreamingServer_RecvMsg_Call[Res] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(interface{}))
	})
	return _c
}

func (_c *MockServerStreamingServer_RecvMsg_Call[Res]) Return(_a0 error) *MockServerStreamingServer_RecvMsg_Call[Res] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockServerStreamingServer_RecvMsg_Call[Res]) RunAndReturn(run func(interface{}) error) *MockServerStreamingServer_RecvMsg_Call[Res] {
	_c.Call.Return(run)
	return _c
}

// Send provides a mock function with given fields: _a0
func (_m *MockServerStreamingServer[Res]) Send(_a0 *testproto.Msg) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Send")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*testproto.Msg) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockServerStreamingServer_Send_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Send'
type MockServerStreamingServer_Send_Call[Res interface{}] struct {
	*mock.Call
}

// Send is a helper method to define mock.On call
//   - _a0 *testproto.Msg
func (_e *MockServerStreamingServer_Expecter[Res]) Send(_a0 interface{}) *MockServerStreamingServer_Send_Call[Res] {
	return &MockServerStreamingServer_Send_Call[Res]{Call: _e.mock.On("Send", _a0)}
}

func (_c *MockServerStreamingServer_Send_Call[Res]) Run(run func(_a0 *testproto.Msg)) *MockServerStreamingServer_Send_Call[Res] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*testproto.Msg))
	})
	return _c
}

func (_c *MockServerStreamingServer_Send_Call[Res]) Return(_a0 error) *MockServerStreamingServer_Send_Call[Res] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockServerStreamingServer_Send_Call[Res]) RunAndReturn(run func(*testproto.Msg) error) *MockServerStreamingServer_Send_Call[Res] {
	_c.Call.Return(run)
	return _c
}

// SendHeader provides a mock function with given fields: _a0
func (_m *MockServerStreamingServer[Res]) SendHeader(_a0 metadata.MD) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SendHeader")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(metadata.MD) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockServerStreamingServer_SendHeader_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SendHeader'
type MockServerStreamingServer_SendHeader_Call[Res interface{}] struct {
	*mock.Call
}

// SendHeader is a helper method to define mock.On call
//   - _a0 metadata.MD
func (_e *MockServerStreamingServer_Expecter[Res]) SendHeader(_a0 interface{}) *MockServerStreamingServer_SendHeader_Call[Res] {
	return &MockServerStreamingServer_SendHeader_Call[Res]{Call: _e.mock.On("SendHeader", _a0)}
}

func (_c *MockServerStreamingServer_SendHeader_Call[Res]) Run(run func(_a0 metadata.MD)) *MockServerStreamingServer_SendHeader_Call[Res] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(metadata.MD))
	})
	return _c
}

func (_c *MockServerStreamingServer_SendHeader_Call[Res]) Return(_a0 error) *MockServerStreamingServer_SendHeader_Call[Res] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockServerStreamingServer_SendHeader_Call[Res]) RunAndReturn(run func(metadata.MD) error) *MockServerStreamingServer_SendHeader_Call[Res] {
	_c.Call.Return(run)
	return _c
}

// SendMsg provides a mock function with given fields: m
func (_m *MockServerStreamingServer[Res]) SendMsg(m interface{}) error {
	ret := _m.Called(m)

	if len(ret) == 0 {
		panic("no return value specified for SendMsg")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockServerStreamingServer_SendMsg_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SendMsg'
type MockServerStreamingServer_SendMsg_Call[Res interface{}] struct {
	*mock.Call
}

// SendMsg is a helper method to define mock.On call
//   - m interface{}
func (_e *MockServerStreamingServer_Expecter[Res]) SendMsg(m interface{}) *MockServerStreamingServer_SendMsg_Call[Res] {
	return &MockServerStreamingServer_SendMsg_Call[Res]{Call: _e.mock.On("SendMsg", m)}
}

func (_c *MockServerStreamingServer_SendMsg_Call[Res]) Run(run func(m interface{})) *MockServerStreamingServer_SendMsg_Call[Res] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(interface{}))
	})
	return _c
}

func (_c *MockServerStreamingServer_SendMsg_Call[Res]) Return(_a0 error) *MockServerStreamingServer_SendMsg_Call[Res] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockServerStreamingServer_SendMsg_Call[Res]) RunAndReturn(run func(interface{}) error) *MockServerStreamingServer_SendMsg_Call[Res] {
	_c.Call.Return(run)
	return _c
}

// SetHeader provides a mock function with given fields: _a0
func (_m *MockServerStreamingServer[Res]) SetHeader(_a0 metadata.MD) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SetHeader")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(metadata.MD) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockServerStreamingServer_SetHeader_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetHeader'
type MockServerStreamingServer_SetHeader_Call[Res interface{}] struct {
	*mock.Call
}

// SetHeader is a helper method to define mock.On call
//   - _a0 metadata.MD
func (_e *MockServerStreamingServer_Expecter[Res]) SetHeader(_a0 interface{}) *MockServerStreamingServer_SetHeader_Call[Res] {
	return &MockServerStreamingServer_SetHeader_Call[Res]{Call: _e.mock.On("SetHeader", _a0)}
}

func (_c *MockServerStreamingServer_SetHeader_Call[Res]) Run(run func(_a0 metadata.MD)) *MockServerStreamingServer_SetHeader_Call[Res] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(metadata.MD))
	})
	return _c
}

func (_c *MockServerStreamingServer_SetHeader_Call[Res]) Return(_a0 error) *MockServerStreamingServer_SetHeader_Call[Res] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockServerStreamingServer_SetHeader_Call[Res]) RunAndReturn(run func(metadata.MD) error) *MockServerStreamingServer_SetHeader_Call[Res] {
	_c.Call.Return(run)
	return _c
}

// SetTrailer provides a mock function with given fields: _a0
func (_m *MockServerStreamingServer[Res]) SetTrailer(_a0 metadata.MD) {
	_m.Called(_a0)
}

// MockServerStreamingServer_SetTrailer_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetTrailer'
type MockServerStreamingServer_SetTrailer_Call[Res interface{}] struct {
	*mock.Call
}

// SetTrailer is a helper method to define mock.On call
//   - _a0 metadata.MD
func (_e *MockServerStreamingServer_Expecter[Res]) SetTrailer(_a0 interface{}) *MockServerStreamingServer_SetTrailer_Call[Res] {
	return &MockServerStreamingServer_SetTrailer_Call[Res]{Call: _e.mock.On("SetTrailer", _a0)}
}

func (_c *MockServerStreamingServer_SetTrailer_Call[Res]) Run(run func(_a0 metadata.MD)) *MockServerStreamingServer_SetTrailer_Call[Res] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(metadata.MD))
	})
	return _c
}

func (_c *MockServerStreamingServer_SetTrailer_Call[Res]) Return() *MockServerStreamingServer_SetTrailer_Call[Res] {
	_c.Call.Return()
	return _c
}

func (_c *MockServerStreamingServer_SetTrailer_Call[Res]) RunAndReturn(run func(metadata.MD)) *MockServerStreamingServer_SetTrailer_Call[Res] {
	_c.Call.Return(run)
	return _c
}

// NewMockServerStreamingServer creates a new instance of MockServerStreamingServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockServerStreamingServer[Res interface{}](t interface {
	mock.TestingT
	Cleanup(func())
}) *MockServerStreamingServer[Res] {
	mock := &MockServerStreamingServer[Res]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
