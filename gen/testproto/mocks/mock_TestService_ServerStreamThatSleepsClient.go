// Code generated by mockery v2.34.2. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	metadata "google.golang.org/grpc/metadata"

	testproto "github.com/avos-io/goat/gen/testproto"
)

// MockTestService_ServerStreamThatSleepsClient is an autogenerated mock type for the TestService_ServerStreamThatSleepsClient type
type MockTestService_ServerStreamThatSleepsClient struct {
	mock.Mock
}

type MockTestService_ServerStreamThatSleepsClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockTestService_ServerStreamThatSleepsClient) EXPECT() *MockTestService_ServerStreamThatSleepsClient_Expecter {
	return &MockTestService_ServerStreamThatSleepsClient_Expecter{mock: &_m.Mock}
}

// CloseSend provides a mock function with given fields:
func (_m *MockTestService_ServerStreamThatSleepsClient) CloseSend() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockTestService_ServerStreamThatSleepsClient_CloseSend_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CloseSend'
type MockTestService_ServerStreamThatSleepsClient_CloseSend_Call struct {
	*mock.Call
}

// CloseSend is a helper method to define mock.On call
func (_e *MockTestService_ServerStreamThatSleepsClient_Expecter) CloseSend() *MockTestService_ServerStreamThatSleepsClient_CloseSend_Call {
	return &MockTestService_ServerStreamThatSleepsClient_CloseSend_Call{Call: _e.mock.On("CloseSend")}
}

func (_c *MockTestService_ServerStreamThatSleepsClient_CloseSend_Call) Run(run func()) *MockTestService_ServerStreamThatSleepsClient_CloseSend_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_CloseSend_Call) Return(_a0 error) *MockTestService_ServerStreamThatSleepsClient_CloseSend_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_CloseSend_Call) RunAndReturn(run func() error) *MockTestService_ServerStreamThatSleepsClient_CloseSend_Call {
	_c.Call.Return(run)
	return _c
}

// Context provides a mock function with given fields:
func (_m *MockTestService_ServerStreamThatSleepsClient) Context() context.Context {
	ret := _m.Called()

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

// MockTestService_ServerStreamThatSleepsClient_Context_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Context'
type MockTestService_ServerStreamThatSleepsClient_Context_Call struct {
	*mock.Call
}

// Context is a helper method to define mock.On call
func (_e *MockTestService_ServerStreamThatSleepsClient_Expecter) Context() *MockTestService_ServerStreamThatSleepsClient_Context_Call {
	return &MockTestService_ServerStreamThatSleepsClient_Context_Call{Call: _e.mock.On("Context")}
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Context_Call) Run(run func()) *MockTestService_ServerStreamThatSleepsClient_Context_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Context_Call) Return(_a0 context.Context) *MockTestService_ServerStreamThatSleepsClient_Context_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Context_Call) RunAndReturn(run func() context.Context) *MockTestService_ServerStreamThatSleepsClient_Context_Call {
	_c.Call.Return(run)
	return _c
}

// Header provides a mock function with given fields:
func (_m *MockTestService_ServerStreamThatSleepsClient) Header() (metadata.MD, error) {
	ret := _m.Called()

	var r0 metadata.MD
	var r1 error
	if rf, ok := ret.Get(0).(func() (metadata.MD, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() metadata.MD); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(metadata.MD)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTestService_ServerStreamThatSleepsClient_Header_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Header'
type MockTestService_ServerStreamThatSleepsClient_Header_Call struct {
	*mock.Call
}

// Header is a helper method to define mock.On call
func (_e *MockTestService_ServerStreamThatSleepsClient_Expecter) Header() *MockTestService_ServerStreamThatSleepsClient_Header_Call {
	return &MockTestService_ServerStreamThatSleepsClient_Header_Call{Call: _e.mock.On("Header")}
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Header_Call) Run(run func()) *MockTestService_ServerStreamThatSleepsClient_Header_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Header_Call) Return(_a0 metadata.MD, _a1 error) *MockTestService_ServerStreamThatSleepsClient_Header_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Header_Call) RunAndReturn(run func() (metadata.MD, error)) *MockTestService_ServerStreamThatSleepsClient_Header_Call {
	_c.Call.Return(run)
	return _c
}

// Recv provides a mock function with given fields:
func (_m *MockTestService_ServerStreamThatSleepsClient) Recv() (*testproto.Msg, error) {
	ret := _m.Called()

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

// MockTestService_ServerStreamThatSleepsClient_Recv_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Recv'
type MockTestService_ServerStreamThatSleepsClient_Recv_Call struct {
	*mock.Call
}

// Recv is a helper method to define mock.On call
func (_e *MockTestService_ServerStreamThatSleepsClient_Expecter) Recv() *MockTestService_ServerStreamThatSleepsClient_Recv_Call {
	return &MockTestService_ServerStreamThatSleepsClient_Recv_Call{Call: _e.mock.On("Recv")}
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Recv_Call) Run(run func()) *MockTestService_ServerStreamThatSleepsClient_Recv_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Recv_Call) Return(_a0 *testproto.Msg, _a1 error) *MockTestService_ServerStreamThatSleepsClient_Recv_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Recv_Call) RunAndReturn(run func() (*testproto.Msg, error)) *MockTestService_ServerStreamThatSleepsClient_Recv_Call {
	_c.Call.Return(run)
	return _c
}

// RecvMsg provides a mock function with given fields: m
func (_m *MockTestService_ServerStreamThatSleepsClient) RecvMsg(m interface{}) error {
	ret := _m.Called(m)

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockTestService_ServerStreamThatSleepsClient_RecvMsg_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RecvMsg'
type MockTestService_ServerStreamThatSleepsClient_RecvMsg_Call struct {
	*mock.Call
}

// RecvMsg is a helper method to define mock.On call
//   - m interface{}
func (_e *MockTestService_ServerStreamThatSleepsClient_Expecter) RecvMsg(m interface{}) *MockTestService_ServerStreamThatSleepsClient_RecvMsg_Call {
	return &MockTestService_ServerStreamThatSleepsClient_RecvMsg_Call{Call: _e.mock.On("RecvMsg", m)}
}

func (_c *MockTestService_ServerStreamThatSleepsClient_RecvMsg_Call) Run(run func(m interface{})) *MockTestService_ServerStreamThatSleepsClient_RecvMsg_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(interface{}))
	})
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_RecvMsg_Call) Return(_a0 error) *MockTestService_ServerStreamThatSleepsClient_RecvMsg_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_RecvMsg_Call) RunAndReturn(run func(interface{}) error) *MockTestService_ServerStreamThatSleepsClient_RecvMsg_Call {
	_c.Call.Return(run)
	return _c
}

// SendMsg provides a mock function with given fields: m
func (_m *MockTestService_ServerStreamThatSleepsClient) SendMsg(m interface{}) error {
	ret := _m.Called(m)

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockTestService_ServerStreamThatSleepsClient_SendMsg_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SendMsg'
type MockTestService_ServerStreamThatSleepsClient_SendMsg_Call struct {
	*mock.Call
}

// SendMsg is a helper method to define mock.On call
//   - m interface{}
func (_e *MockTestService_ServerStreamThatSleepsClient_Expecter) SendMsg(m interface{}) *MockTestService_ServerStreamThatSleepsClient_SendMsg_Call {
	return &MockTestService_ServerStreamThatSleepsClient_SendMsg_Call{Call: _e.mock.On("SendMsg", m)}
}

func (_c *MockTestService_ServerStreamThatSleepsClient_SendMsg_Call) Run(run func(m interface{})) *MockTestService_ServerStreamThatSleepsClient_SendMsg_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(interface{}))
	})
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_SendMsg_Call) Return(_a0 error) *MockTestService_ServerStreamThatSleepsClient_SendMsg_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_SendMsg_Call) RunAndReturn(run func(interface{}) error) *MockTestService_ServerStreamThatSleepsClient_SendMsg_Call {
	_c.Call.Return(run)
	return _c
}

// Trailer provides a mock function with given fields:
func (_m *MockTestService_ServerStreamThatSleepsClient) Trailer() metadata.MD {
	ret := _m.Called()

	var r0 metadata.MD
	if rf, ok := ret.Get(0).(func() metadata.MD); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(metadata.MD)
		}
	}

	return r0
}

// MockTestService_ServerStreamThatSleepsClient_Trailer_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Trailer'
type MockTestService_ServerStreamThatSleepsClient_Trailer_Call struct {
	*mock.Call
}

// Trailer is a helper method to define mock.On call
func (_e *MockTestService_ServerStreamThatSleepsClient_Expecter) Trailer() *MockTestService_ServerStreamThatSleepsClient_Trailer_Call {
	return &MockTestService_ServerStreamThatSleepsClient_Trailer_Call{Call: _e.mock.On("Trailer")}
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Trailer_Call) Run(run func()) *MockTestService_ServerStreamThatSleepsClient_Trailer_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Trailer_Call) Return(_a0 metadata.MD) *MockTestService_ServerStreamThatSleepsClient_Trailer_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockTestService_ServerStreamThatSleepsClient_Trailer_Call) RunAndReturn(run func() metadata.MD) *MockTestService_ServerStreamThatSleepsClient_Trailer_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockTestService_ServerStreamThatSleepsClient creates a new instance of MockTestService_ServerStreamThatSleepsClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockTestService_ServerStreamThatSleepsClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockTestService_ServerStreamThatSleepsClient {
	mock := &MockTestService_ServerStreamThatSleepsClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
