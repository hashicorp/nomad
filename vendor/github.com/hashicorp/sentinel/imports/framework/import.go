// Package framework contains a high-level framework for implementing
// Sentinel imports with Go.
//
// The direct runtime/gobridge.Import interface is a low-level interface
// that is tediuos, clunky, and difficult to implement correctly. The interface
// is this way to assist in the performance of imports while executing
// Sentinel policies. This package provides a high-level API that eases
// import implementation while still supporting the performance-sensitive
// interface underneath.
package framework

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/sentinel/lang/object"
	"github.com/hashicorp/sentinel/runtime/encoding"
	"github.com/hashicorp/sentinel/runtime/gobridge"
)

// Import implements gobridge.Import. Configure and return this structure
// to simplify implementation of gobridge.Import.
type Import struct {
	// Root is the implementation of the import that the user of the
	// framework should implement. It represents the minimum necessary
	// implementation for an import. See the docs for Root for more details.
	Root Root

	// namespaceMap keeps track of all the Namespaces for the various
	// executions. These are cleaned up based on the ExecDeadline.
	namespaceMap  map[uint64]Namespace
	namespaceLock sync.RWMutex
}

// plugin.Import impl.
func (m *Import) Configure(raw map[string]interface{}) error {
	// Verify the root implementation is a Namespace or NamespaceCreator.
	switch m.Root.(type) {
	case Namespace:
	case NamespaceCreator:
	default:
		return fmt.Errorf("invalid import implementation, please report a " +
			"bug to the developer of this import")
	}

	// Configure the object itself
	return m.Root.Configure(raw)
}

// plugin.Import impl.
func (m *Import) Get(reqs []*gobridge.GetReq) ([]*gobridge.GetResult, error) {
	resp := make([]*gobridge.GetResult, len(reqs))
	for i, req := range reqs {
		// Get the namespace
		ns := m.namespace(req)

		// Is this a call?
		call := req.Args != nil

		// For each key, perform a get
		var result interface{} = ns
		for i, k := range req.Keys {
			// If this is the last key in a call, then we have to perform
			// the actual function call here.
			if call && i == len(req.Keys)-1 {
				x, ok := result.(Call)
				if !ok {
					return nil, fmt.Errorf(
						"key %q doesn't support function calls",
						strings.Join(req.Keys[:i], "."))
				}

				v, err := m.call(x.Func(k), req.Args)
				if err != nil {
					return nil, fmt.Errorf(
						"error calling function %q: %s",
						strings.Join(req.Keys[:i], "."), err)
				}

				result = v
				break
			}

			switch x := result.(type) {
			// For namespaces, we get the next value in the chain
			case Namespace:
				v, err := x.Get(k)
				if err != nil {
					return nil, fmt.Errorf(
						"error retrieving key %q: %s",
						strings.Join(req.Keys[:i], "."), err)
				}

				result = v

			// For maps with string keys, get the value
			case map[string]interface{}:
				result = x[k]

			// For any other type, the result is undefined
			default:
				result = nil
			}

			if result == nil {
				break
			}
		}

		// If we have a Map implementation, we return the whole thing.
		if m, ok := result.(Map); ok {
			var err error
			result, err = m.Map()
			if err != nil {
				return nil, fmt.Errorf(
					"error retrieving key %q: %s",
					strings.Join(req.Keys, "."), err)
			}
		}

		// If our result is a map[string]interface{}, we automatically
		// convert namespace results that implement Map into their
		// respective maps. We do this recursively.
		if v := reflect.ValueOf(result); v.Kind() == reflect.Map {
			var err error
			result, err = flattenMapValues(v)
			if err != nil {
				return nil, fmt.Errorf(
					"error retrieving key %q: %s",
					strings.Join(req.Keys, "."), err)
			}
		}

		// Convert the result based on types
		if result == nil {
			result = &object.UndefinedObj{}
		}

		// Build the actual result
		resp[i] = &gobridge.GetResult{
			KeyId: req.KeyId,
			Keys:  req.Keys,
			Value: result,
		}
	}

	return resp, nil
}

// namespace returns the namespace for the request.
func (m *Import) namespace(req *gobridge.GetReq) Namespace {
	if global, ok := m.Root.(Namespace); ok {
		return global
	}

	// Look for it in the cache of executions
	m.namespaceLock.RLock()
	ns, ok := m.namespaceMap[req.ExecId]
	m.namespaceLock.RUnlock()
	if ok {
		return ns
	}

	nsFunc, ok := m.Root.(NamespaceCreator)
	if !ok {
		panic("Root must be NamespaceCreator if not Namespace")
	}

	// Not found, we have to create it
	m.namespaceLock.Lock()
	defer m.namespaceLock.Unlock()

	// If it was created while we didn't have the lock, return it
	ns, ok = m.namespaceMap[req.ExecId]
	if ok {
		return ns
	}

	// Init if we have to
	if m.namespaceMap == nil {
		m.namespaceMap = make(map[uint64]Namespace)
	}

	// Create it
	ns = nsFunc.Namespace()
	m.namespaceMap[req.ExecId] = ns

	// Create the expiration function
	time.AfterFunc(time.Until(req.ExecDeadline), func() {
		m.invalidateNamespace(req.ExecId)
	})

	return ns
}

func (m *Import) invalidateNamespace(id uint64) {
	m.namespaceLock.Lock()
	defer m.namespaceLock.Unlock()
	delete(m.namespaceMap, id)
}

// call performs the typed function call via reflection for f.
func (m *Import) call(f interface{}, args []object.Object) (interface{}, error) {
	// If a function call isn't supported for this key, then it is an error
	if f == nil {
		return nil, fmt.Errorf("function call unsupported")
	}

	// Reflect on the function and verify it is a function
	funcVal := reflect.ValueOf(f)
	if funcVal.Kind() != reflect.Func {
		return nil, fmt.Errorf(
			"internal error: import didn't return function for key")
	}
	funcType := funcVal.Type()

	// Verify argument count
	if len(args) != funcType.NumIn() {
		return nil, fmt.Errorf(
			"expected %d arguments, got %d",
			funcType.NumIn(), len(args))
	}

	// Go through the arguments and convert them to the proper type
	funcArgs := make([]reflect.Value, funcType.NumIn())
	for i := 0; i < funcType.NumIn(); i++ {
		t := funcType.In(i)

		// If the argument type is directly the object then use it as-is
		if t == objectType {
			funcArgs[i] = reflect.ValueOf(args[i])
			continue
		}

		// Convert the argument to the desired type
		arg, err := encoding.ObjectToGo(args[i], t)
		if err != nil {
			return nil, err
		}

		funcArgs[i] = reflect.ValueOf(arg)
	}

	// Call the function
	funcRets := funcVal.Call(funcArgs)

	// Build the return values
	var err error
	if len(funcRets) > 1 {
		if v := funcRets[1].Interface(); v != nil {
			err = v.(error)
		}
	}

	return funcRets[0].Interface(), err
}

var objectType = reflect.TypeOf((*object.Object)(nil)).Elem()
