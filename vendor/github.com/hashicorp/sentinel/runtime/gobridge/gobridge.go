// Package gobridge contains the interfaces for bridging Go code to Sentinel
// code. These are the primary interfaces that should be implemented for
// writing Go code that is invokable or accessible from Sentinel.
package gobridge

import (
	"time"

	"github.com/hashicorp/sentinel/lang/object"
)

// Import is an importable package.
//
// As of writing, imports can only be implemented by Go so this is also the
// only way to implement imports for Sentinel. You may also serve imports
// across external plugins. See the "runtime/plugin" and "runtime/plugin/rpc"
// package.
//
// Imports are a namespace that may contain objects and functions. The
// root level has no value nor can it be called. For example `import "a'`
// allows access to fields within "a" such as `a.b` but doesn't allow
// referencing or calling it directly with `a` alone. This is an important
// difference between imports and external objects (which themselves express
// a value).
type Import interface {
	// Get is called when an import field is accessed or called as a function.
	//
	// Get may request more than one value at a time, represented by multiple
	// GetReq values. The result GetResult should contain the matching
	// KeyId for the requests.
	//
	// The result value is not a map keyed on KeyId to allow flexibility
	// in the future of potentially allowing pre-fetched data. This has no
	// effect currently.
	Get(reqs []*GetReq) ([]*GetResult, error)
}

// GetReq are the arguments given to Get for an Import.
type GetReq struct {
	// ExecId is a unique ID representing the particular execution for this
	// request. This can be used to maintain state locally.
	//
	// ExecDeadline is a hint of when the execution will be over and the
	// state can be thrown away. The time given here will always be in UTC
	// time. Note that this is susceptible to clock shifts, but Go is planning
	// to make the time APIs monotonic by default (see proposal 12914). After
	// that this will be resolved.
	ExecId       uint64
	ExecDeadline time.Time

	// Keys is the list of keys being requested. For example for "a.b.c"
	// where "a" is the import, Keys would be ["b", "c"].
	//
	// KeyId is a unique ID for this key. This should match exactly the
	// GetResult KeyId so that the result for this can be found quickly.
	Keys  []string
	KeyId uint64

	// Args is the list of arguments for a call expression. This is "nil"
	// if this isn't a call. This may be length zero (but non-nil) if this
	// is a call with no arguments.
	Args []object.Object
}

// Call returns true if this request is a call expression.
func (g *GetReq) Call() bool {
	return g.Args != nil
}

// GetResult is the result structure for a Get request.
type GetResult struct {
	KeyId uint64      // KeyId matching GetReq.KeyId, or zero.
	Keys  []string    // Keys structure from GetReq.Keys, or new key set.
	Value interface{} // Value compatible with lang/object.ToObject
}

// GetResultList is a wrapper around a slice of GetResult structures
// to provide helpers.
type GetResultList []*GetResult

// KeyId gets the result with the given key ID, or nil if its not found.
func (r GetResultList) KeyId(id uint64) *GetResult {
	for _, v := range r {
		if v.KeyId == id {
			return v
		}
	}

	return nil
}
