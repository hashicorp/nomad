package object

import (
	"fmt"
)

// External is an interface for sourcing data from an external data source.
// The data could come from Go, from across the network, out of process, etc.
// This data is automatically converted into the object model that Sentinel
// expects.
//
// The bare interface is very simple. Runtime implementations, semantic passes,
// and others may request a richer interface that embeds this for optimizations.
type External interface {
	// Call is called if this external object is treated like a function call.
	// The arguments are the native Object implementations.
	Call(args []Object) (interface{}, error)

	// Get reads the value for the given key. This value will be cached.
	// The return value may be any Go primitives or Sentinel Object
	// implementations.
	Get(string) (interface{}, error)
}

// ExternalMap can be embedded in a structure implementing External to
// force it to only support the Get operation as a map.
type ExternalMap struct{}

func (ExternalMap) Call([]Object) (interface{}, error) {
	return nil, fmt.Errorf("object cannot be called")
}

// ExternalFunc wraps a function to turn it into an External.
func ExternalFunc(f func([]Object) (interface{}, error)) Object {
	return &ExternalObj{External: externalFunc(f)}
}

type externalFunc func([]Object) (interface{}, error)

func (f externalFunc) Call(args []Object) (interface{}, error) {
	return f(args)
}

func (externalFunc) Get(string) (interface{}, error) {
	return nil, fmt.Errorf("object is a function, not a map")
}

//------------------------------------------------------------------------
// Mock

// MockExternal implements External but has the methods mocked out.
// This is meant to be used only for tests.
type MockExternal struct {
	CallCalled    bool
	CallArgs      []Object
	CallReturn    interface{}
	CallReturnErr error
	CallFunc      func([]Object) (interface{}, error)

	GetCalled    bool
	GetKey       string
	GetReturn    interface{}
	GetReturnErr error
}

func (e *MockExternal) Call(args []Object) (interface{}, error) {
	e.CallCalled = true
	e.CallArgs = args

	if e.CallFunc != nil {
		return e.CallFunc(args)
	}

	return e.CallReturn, e.CallReturnErr
}

func (e *MockExternal) Get(key string) (interface{}, error) {
	e.GetCalled = true
	e.GetKey = key
	return e.GetReturn, e.GetReturnErr
}
