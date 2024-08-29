// Code generated by mockery v2.45.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	metadata "google.golang.org/grpc/metadata"

	testproto "github.com/avos-io/goat/gen/testproto"
)

// MockTestService_BidiStreamServer is an autogenerated mock type for the TestService_BidiStreamServer type
type MockTestService_BidiStreamServer struct {
	mock.Mock
}

type MockTestService_BidiStreamServer_Expecter struct {
	mock *mock.Mock
}

func (_m *MockTestService_BidiStreamServer) EXPECT() *MockTestService_BidiStreamServer_Expecter {
	return &MockTestService_BidiStreamServer_Expecter{mock: &_m.Mock}
}

// Context provides a mock function with given fields:
func (_m *MockTestService_BidiStreamServer) Context() context.Context {
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

// MockTestService_BidiStreamServer_Context_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Context'
type MockTestService_BidiStreamServer_Context_Call struct {
	*mock.Call
}

// Context is a helper method to define mock.On call
func (_e *MockTestService_BidiStreamServer_Expecter) Context() *MockTestService_BidiStreamServer_Context_Call {
	return &MockTestService_BidiStreamServer_Context_Call{Call: _e.mock.On("Context")}
}

func (_c *MockTestService_BidiStreamServer_Context_Call) Run(run func()) *MockTestService_BidiStreamServer_Context_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockTestService_BidiStreamServer_Context_Call) Return(_a0 context.Context) *MockTestService_BidiStreamServer_Context_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_BidiStreamServer_Context_Call) RunAndReturn(run func() context.Context) *MockTestService_BidiStreamServer_Context_Call {
	_c.Call.Return(run)
	return _c
}

// Recv provides a mock function with given fields:
func (_m *MockTestService_BidiStreamServer) Recv() (*testproto.Msg, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Recv")
	}

	var r0 *testproto.Msg
	var r1 error
	if rf, ok := ret.Get(0).(func() (*testproto.Msg, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *testproto.Msg); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*testproto.Msg)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTestService_BidiStreamServer_Recv_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Recv'
type MockTestService_BidiStreamServer_Recv_Call struct {
	*mock.Call
}

// Recv is a helper method to define mock.On call
func (_e *MockTestService_BidiStreamServer_Expecter) Recv() *MockTestService_BidiStreamServer_Recv_Call {
	return &MockTestService_BidiStreamServer_Recv_Call{Call: _e.mock.On("Recv")}
}

func (_c *MockTestService_BidiStreamServer_Recv_Call) Run(run func()) *MockTestService_BidiStreamServer_Recv_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockTestService_BidiStreamServer_Recv_Call) Return(_a0 *testproto.Msg, _a1 error) *MockTestService_BidiStreamServer_Recv_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTestService_BidiStreamServer_Recv_Call) RunAndReturn(run func() (*testproto.Msg, error)) *MockTestService_BidiStreamServer_Recv_Call {
	_c.Call.Return(run)
	return _c
}

// RecvMsg provides a mock function with given fields: m
func (_m *MockTestService_BidiStreamServer) RecvMsg(m interface{}) error {
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

// MockTestService_BidiStreamServer_RecvMsg_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RecvMsg'
type MockTestService_BidiStreamServer_RecvMsg_Call struct {
	*mock.Call
}

// RecvMsg is a helper method to define mock.On call
//   - m interface{}
func (_e *MockTestService_BidiStreamServer_Expecter) RecvMsg(m interface{}) *MockTestService_BidiStreamServer_RecvMsg_Call {
	return &MockTestService_BidiStreamServer_RecvMsg_Call{Call: _e.mock.On("RecvMsg", m)}
}

func (_c *MockTestService_BidiStreamServer_RecvMsg_Call) Run(run func(m interface{})) *MockTestService_BidiStreamServer_RecvMsg_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(interface{}))
	})
	return _c
}

func (_c *MockTestService_BidiStreamServer_RecvMsg_Call) Return(_a0 error) *MockTestService_BidiStreamServer_RecvMsg_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_BidiStreamServer_RecvMsg_Call) RunAndReturn(run func(interface{}) error) *MockTestService_BidiStreamServer_RecvMsg_Call {
	_c.Call.Return(run)
	return _c
}

// Send provides a mock function with given fields: _a0
func (_m *MockTestService_BidiStreamServer) Send(_a0 *testproto.Msg) error {
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

// MockTestService_BidiStreamServer_Send_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Send'
type MockTestService_BidiStreamServer_Send_Call struct {
	*mock.Call
}

// Send is a helper method to define mock.On call
//   - _a0 *testproto.Msg
func (_e *MockTestService_BidiStreamServer_Expecter) Send(_a0 interface{}) *MockTestService_BidiStreamServer_Send_Call {
	return &MockTestService_BidiStreamServer_Send_Call{Call: _e.mock.On("Send", _a0)}
}

func (_c *MockTestService_BidiStreamServer_Send_Call) Run(run func(_a0 *testproto.Msg)) *MockTestService_BidiStreamServer_Send_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*testproto.Msg))
	})
	return _c
}

func (_c *MockTestService_BidiStreamServer_Send_Call) Return(_a0 error) *MockTestService_BidiStreamServer_Send_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_BidiStreamServer_Send_Call) RunAndReturn(run func(*testproto.Msg) error) *MockTestService_BidiStreamServer_Send_Call {
	_c.Call.Return(run)
	return _c
}

// SendHeader provides a mock function with given fields: _a0
func (_m *MockTestService_BidiStreamServer) SendHeader(_a0 metadata.MD) error {
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

// MockTestService_BidiStreamServer_SendHeader_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SendHeader'
type MockTestService_BidiStreamServer_SendHeader_Call struct {
	*mock.Call
}

// SendHeader is a helper method to define mock.On call
//   - _a0 metadata.MD
func (_e *MockTestService_BidiStreamServer_Expecter) SendHeader(_a0 interface{}) *MockTestService_BidiStreamServer_SendHeader_Call {
	return &MockTestService_BidiStreamServer_SendHeader_Call{Call: _e.mock.On("SendHeader", _a0)}
}

func (_c *MockTestService_BidiStreamServer_SendHeader_Call) Run(run func(_a0 metadata.MD)) *MockTestService_BidiStreamServer_SendHeader_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(metadata.MD))
	})
	return _c
}

func (_c *MockTestService_BidiStreamServer_SendHeader_Call) Return(_a0 error) *MockTestService_BidiStreamServer_SendHeader_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_BidiStreamServer_SendHeader_Call) RunAndReturn(run func(metadata.MD) error) *MockTestService_BidiStreamServer_SendHeader_Call {
	_c.Call.Return(run)
	return _c
}

// SendMsg provides a mock function with given fields: m
func (_m *MockTestService_BidiStreamServer) SendMsg(m interface{}) error {
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

// MockTestService_BidiStreamServer_SendMsg_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SendMsg'
type MockTestService_BidiStreamServer_SendMsg_Call struct {
	*mock.Call
}

// SendMsg is a helper method to define mock.On call
//   - m interface{}
func (_e *MockTestService_BidiStreamServer_Expecter) SendMsg(m interface{}) *MockTestService_BidiStreamServer_SendMsg_Call {
	return &MockTestService_BidiStreamServer_SendMsg_Call{Call: _e.mock.On("SendMsg", m)}
}

func (_c *MockTestService_BidiStreamServer_SendMsg_Call) Run(run func(m interface{})) *MockTestService_BidiStreamServer_SendMsg_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(interface{}))
	})
	return _c
}

func (_c *MockTestService_BidiStreamServer_SendMsg_Call) Return(_a0 error) *MockTestService_BidiStreamServer_SendMsg_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_BidiStreamServer_SendMsg_Call) RunAndReturn(run func(interface{}) error) *MockTestService_BidiStreamServer_SendMsg_Call {
	_c.Call.Return(run)
	return _c
}

// SetHeader provides a mock function with given fields: _a0
func (_m *MockTestService_BidiStreamServer) SetHeader(_a0 metadata.MD) error {
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

// MockTestService_BidiStreamServer_SetHeader_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetHeader'
type MockTestService_BidiStreamServer_SetHeader_Call struct {
	*mock.Call
}

// SetHeader is a helper method to define mock.On call
//   - _a0 metadata.MD
func (_e *MockTestService_BidiStreamServer_Expecter) SetHeader(_a0 interface{}) *MockTestService_BidiStreamServer_SetHeader_Call {
	return &MockTestService_BidiStreamServer_SetHeader_Call{Call: _e.mock.On("SetHeader", _a0)}
}

func (_c *MockTestService_BidiStreamServer_SetHeader_Call) Run(run func(_a0 metadata.MD)) *MockTestService_BidiStreamServer_SetHeader_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(metadata.MD))
	})
	return _c
}

func (_c *MockTestService_BidiStreamServer_SetHeader_Call) Return(_a0 error) *MockTestService_BidiStreamServer_SetHeader_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_BidiStreamServer_SetHeader_Call) RunAndReturn(run func(metadata.MD) error) *MockTestService_BidiStreamServer_SetHeader_Call {
	_c.Call.Return(run)
	return _c
}

// SetTrailer provides a mock function with given fields: _a0
func (_m *MockTestService_BidiStreamServer) SetTrailer(_a0 metadata.MD) {
	_m.Called(_a0)
}

// MockTestService_BidiStreamServer_SetTrailer_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetTrailer'
type MockTestService_BidiStreamServer_SetTrailer_Call struct {
	*mock.Call
}

// SetTrailer is a helper method to define mock.On call
//   - _a0 metadata.MD
func (_e *MockTestService_BidiStreamServer_Expecter) SetTrailer(_a0 interface{}) *MockTestService_BidiStreamServer_SetTrailer_Call {
	return &MockTestService_BidiStreamServer_SetTrailer_Call{Call: _e.mock.On("SetTrailer", _a0)}
}

func (_c *MockTestService_BidiStreamServer_SetTrailer_Call) Run(run func(_a0 metadata.MD)) *MockTestService_BidiStreamServer_SetTrailer_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(metadata.MD))
	})
	return _c
}

func (_c *MockTestService_BidiStreamServer_SetTrailer_Call) Return() *MockTestService_BidiStreamServer_SetTrailer_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockTestService_BidiStreamServer_SetTrailer_Call) RunAndReturn(run func(metadata.MD)) *MockTestService_BidiStreamServer_SetTrailer_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockTestService_BidiStreamServer creates a new instance of MockTestService_BidiStreamServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockTestService_BidiStreamServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockTestService_BidiStreamServer {
	mock := &MockTestService_BidiStreamServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
