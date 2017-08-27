// Package static contains a Sentinel plugin for serving state data into
// a Sentinel policy via an import.
package static

import (
	"reflect"

	"github.com/hashicorp/sentinel/imports/framework"
	"github.com/hashicorp/sentinel/lang/object"
	"github.com/hashicorp/sentinel/runtime/encoding"
)

// New creates a new Import.
func New() *framework.Import {
	return &framework.Import{
		Root: &root{},
	}
}

// NewObject creates a static object for a given Go value. This may
// return a RuntimeObj with a Value that is a gobridge.Import. Therefore,
// this function should only be used with the runtime evaluator.
func NewObject(raw interface{}) (object.Object, error) {
	v := recurseReturn(raw)
	if ns, ok := v.(framework.Namespace); ok {
		return &object.RuntimeObj{
			Value: &framework.Import{Root: &root{Namespace: ns}},
		}, nil
	}

	return encoding.GoToObject(raw)
}

// NewMap is a helper that returns a configured import for the given map.
func NewMap(m map[string]interface{}) (*framework.Import, error) {
	impt := &framework.Import{Root: &root{}}
	return impt, impt.Configure(m)
}

// NewStruct is a helper that returns a configured import for the given struct.
func NewStruct(v reflect.Value) (*framework.Import, error) {
	r := &root{Namespace: &structNS{value: v, original: v}}
	impt := &framework.Import{Root: r}
	return impt, nil
}

type root struct {
	framework.Namespace
}

// framework.Root impl.
func (m *root) Configure(raw map[string]interface{}) error {
	// Set our root namespace
	m.Namespace = &mapNS{objects: raw}

	return nil
}

// recurseReturn can be called on a resulting value to determine what
// further recursion can be done to the right namespace.
func recurseReturn(raw interface{}) interface{} {
	// If the value is a map, recurse further by returning a namespace
	if m, ok := raw.(map[string]interface{}); ok {
		return &mapNS{objects: m}
	}

	// If the value is a struct, setup a struct lookup
	originalVal := reflect.ValueOf(raw)
	v := originalVal
	for v.Kind() == reflect.Ptr {
		v = reflect.Indirect(v)
	}
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Struct {
		return &structNS{
			value:    v,
			original: originalVal,
		}
	}

	return raw
}
