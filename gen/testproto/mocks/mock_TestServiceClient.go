// Code generated by mockery v2.45.0. DO NOT EDIT.

package mocks

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"

	testproto "github.com/avos-io/goat/gen/testproto"
)

// MockTestServiceClient is an autogenerated mock type for the TestServiceClient type
type MockTestServiceClient struct {
	mock.Mock
}

type MockTestServiceClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockTestServiceClient) EXPECT() *MockTestServiceClient_Expecter {
	return &MockTestServiceClient_Expecter{mock: &_m.Mock}
}

// BidiStream provides a mock function with given fields: ctx, opts
func (_m *MockTestServiceClient) BidiStream(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[testproto.Msg, testproto.Msg], error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for BidiStream")
	}

	var r0 grpc.BidiStreamingClient[testproto.Msg, testproto.Msg]
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[testproto.Msg, testproto.Msg], error)); ok {
		return rf(ctx, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, ...grpc.CallOption) grpc.BidiStreamingClient[testproto.Msg, testproto.Msg]); ok {
		r0 = rf(ctx, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(grpc.BidiStreamingClient[testproto.Msg, testproto.Msg])
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTestServiceClient_BidiStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BidiStream'
type MockTestServiceClient_BidiStream_Call struct {
	*mock.Call
}

// BidiStream is a helper method to define mock.On call
//   - ctx context.Context
//   - opts ...grpc.CallOption
func (_e *MockTestServiceClient_Expecter) BidiStream(ctx interface{}, opts ...interface{}) *MockTestServiceClient_BidiStream_Call {
	return &MockTestServiceClient_BidiStream_Call{Call: _e.mock.On("BidiStream",
		append([]interface{}{ctx}, opts...)...)}
}

func (_c *MockTestServiceClient_BidiStream_Call) Run(run func(ctx context.Context, opts ...grpc.CallOption)) *MockTestServiceClient_BidiStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-1)
		for i, a := range args[1:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), variadicArgs...)
	})
	return _c
}

func (_c *MockTestServiceClient_BidiStream_Call) Return(_a0 grpc.BidiStreamingClient[testproto.Msg, testproto.Msg], _a1 error) *MockTestServiceClient_BidiStream_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTestServiceClient_BidiStream_Call) RunAndReturn(run func(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[testproto.Msg, testproto.Msg], error)) *MockTestServiceClient_BidiStream_Call {
	_c.Call.Return(run)
	return _c
}

// ClientStream provides a mock function with given fields: ctx, opts
func (_m *MockTestServiceClient) ClientStream(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[testproto.Msg, testproto.Msg], error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for ClientStream")
	}

	var r0 grpc.ClientStreamingClient[testproto.Msg, testproto.Msg]
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, ...grpc.CallOption) (grpc.ClientStreamingClient[testproto.Msg, testproto.Msg], error)); ok {
		return rf(ctx, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, ...grpc.CallOption) grpc.ClientStreamingClient[testproto.Msg, testproto.Msg]); ok {
		r0 = rf(ctx, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(grpc.ClientStreamingClient[testproto.Msg, testproto.Msg])
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTestServiceClient_ClientStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ClientStream'
type MockTestServiceClient_ClientStream_Call struct {
	*mock.Call
}

// ClientStream is a helper method to define mock.On call
//   - ctx context.Context
//   - opts ...grpc.CallOption
func (_e *MockTestServiceClient_Expecter) ClientStream(ctx interface{}, opts ...interface{}) *MockTestServiceClient_ClientStream_Call {
	return &MockTestServiceClient_ClientStream_Call{Call: _e.mock.On("ClientStream",
		append([]interface{}{ctx}, opts...)...)}
}

func (_c *MockTestServiceClient_ClientStream_Call) Run(run func(ctx context.Context, opts ...grpc.CallOption)) *MockTestServiceClient_ClientStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-1)
		for i, a := range args[1:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), variadicArgs...)
	})
	return _c
}

func (_c *MockTestServiceClient_ClientStream_Call) Return(_a0 grpc.ClientStreamingClient[testproto.Msg, testproto.Msg], _a1 error) *MockTestServiceClient_ClientStream_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTestServiceClient_ClientStream_Call) RunAndReturn(run func(context.Context, ...grpc.CallOption) (grpc.ClientStreamingClient[testproto.Msg, testproto.Msg], error)) *MockTestServiceClient_ClientStream_Call {
	_c.Call.Return(run)
	return _c
}

// ServerStream provides a mock function with given fields: ctx, in, opts
func (_m *MockTestServiceClient) ServerStream(ctx context.Context, in *testproto.Msg, opts ...grpc.CallOption) (grpc.ServerStreamingClient[testproto.Msg], error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for ServerStream")
	}

	var r0 grpc.ServerStreamingClient[testproto.Msg]
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *testproto.Msg, ...grpc.CallOption) (grpc.ServerStreamingClient[testproto.Msg], error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *testproto.Msg, ...grpc.CallOption) grpc.ServerStreamingClient[testproto.Msg]); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(grpc.ServerStreamingClient[testproto.Msg])
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *testproto.Msg, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTestServiceClient_ServerStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ServerStream'
type MockTestServiceClient_ServerStream_Call struct {
	*mock.Call
}

// ServerStream is a helper method to define mock.On call
//   - ctx context.Context
//   - in *testproto.Msg
//   - opts ...grpc.CallOption
func (_e *MockTestServiceClient_Expecter) ServerStream(ctx interface{}, in interface{}, opts ...interface{}) *MockTestServiceClient_ServerStream_Call {
	return &MockTestServiceClient_ServerStream_Call{Call: _e.mock.On("ServerStream",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockTestServiceClient_ServerStream_Call) Run(run func(ctx context.Context, in *testproto.Msg, opts ...grpc.CallOption)) *MockTestServiceClient_ServerStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*testproto.Msg), variadicArgs...)
	})
	return _c
}

func (_c *MockTestServiceClient_ServerStream_Call) Return(_a0 grpc.ServerStreamingClient[testproto.Msg], _a1 error) *MockTestServiceClient_ServerStream_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTestServiceClient_ServerStream_Call) RunAndReturn(run func(context.Context, *testproto.Msg, ...grpc.CallOption) (grpc.ServerStreamingClient[testproto.Msg], error)) *MockTestServiceClient_ServerStream_Call {
	_c.Call.Return(run)
	return _c
}

// ServerStreamThatSleeps provides a mock function with given fields: ctx, in, opts
func (_m *MockTestServiceClient) ServerStreamThatSleeps(ctx context.Context, in *testproto.Msg, opts ...grpc.CallOption) (grpc.ServerStreamingClient[testproto.Msg], error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for ServerStreamThatSleeps")
	}

	var r0 grpc.ServerStreamingClient[testproto.Msg]
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *testproto.Msg, ...grpc.CallOption) (grpc.ServerStreamingClient[testproto.Msg], error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *testproto.Msg, ...grpc.CallOption) grpc.ServerStreamingClient[testproto.Msg]); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(grpc.ServerStreamingClient[testproto.Msg])
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *testproto.Msg, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTestServiceClient_ServerStreamThatSleeps_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ServerStreamThatSleeps'
type MockTestServiceClient_ServerStreamThatSleeps_Call struct {
	*mock.Call
}

// ServerStreamThatSleeps is a helper method to define mock.On call
//   - ctx context.Context
//   - in *testproto.Msg
//   - opts ...grpc.CallOption
func (_e *MockTestServiceClient_Expecter) ServerStreamThatSleeps(ctx interface{}, in interface{}, opts ...interface{}) *MockTestServiceClient_ServerStreamThatSleeps_Call {
	return &MockTestServiceClient_ServerStreamThatSleeps_Call{Call: _e.mock.On("ServerStreamThatSleeps",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockTestServiceClient_ServerStreamThatSleeps_Call) Run(run func(ctx context.Context, in *testproto.Msg, opts ...grpc.CallOption)) *MockTestServiceClient_ServerStreamThatSleeps_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*testproto.Msg), variadicArgs...)
	})
	return _c
}

func (_c *MockTestServiceClient_ServerStreamThatSleeps_Call) Return(_a0 grpc.ServerStreamingClient[testproto.Msg], _a1 error) *MockTestServiceClient_ServerStreamThatSleeps_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTestServiceClient_ServerStreamThatSleeps_Call) RunAndReturn(run func(context.Context, *testproto.Msg, ...grpc.CallOption) (grpc.ServerStreamingClient[testproto.Msg], error)) *MockTestServiceClient_ServerStreamThatSleeps_Call {
	_c.Call.Return(run)
	return _c
}

// Unary provides a mock function with given fields: ctx, in, opts
func (_m *MockTestServiceClient) Unary(ctx context.Context, in *testproto.Msg, opts ...grpc.CallOption) (*testproto.Msg, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Unary")
	}

	var r0 *testproto.Msg
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *testproto.Msg, ...grpc.CallOption) (*testproto.Msg, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *testproto.Msg, ...grpc.CallOption) *testproto.Msg); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*testproto.Msg)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *testproto.Msg, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTestServiceClient_Unary_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Unary'
type MockTestServiceClient_Unary_Call struct {
	*mock.Call
}

// Unary is a helper method to define mock.On call
//   - ctx context.Context
//   - in *testproto.Msg
//   - opts ...grpc.CallOption
func (_e *MockTestServiceClient_Expecter) Unary(ctx interface{}, in interface{}, opts ...interface{}) *MockTestServiceClient_Unary_Call {
	return &MockTestServiceClient_Unary_Call{Call: _e.mock.On("Unary",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockTestServiceClient_Unary_Call) Run(run func(ctx context.Context, in *testproto.Msg, opts ...grpc.CallOption)) *MockTestServiceClient_Unary_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*testproto.Msg), variadicArgs...)
	})
	return _c
}

func (_c *MockTestServiceClient_Unary_Call) Return(_a0 *testproto.Msg, _a1 error) *MockTestServiceClient_Unary_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTestServiceClient_Unary_Call) RunAndReturn(run func(context.Context, *testproto.Msg, ...grpc.CallOption) (*testproto.Msg, error)) *MockTestServiceClient_Unary_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockTestServiceClient creates a new instance of MockTestServiceClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockTestServiceClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockTestServiceClient {
	mock := &MockTestServiceClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
