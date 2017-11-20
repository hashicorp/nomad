// Package static contains a Sentinel plugin for serving state data into
// a Sentinel policy via an import.
package static

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/hashicorp/sentinel-sdk"
	"github.com/hashicorp/sentinel-sdk/framework"
	"github.com/hashicorp/sentinel/lang/object"
	"github.com/hashicorp/sentinel/runtime/encoding"

	"github.com/hashicorp/sentinel/imports/static/teststructs"
)

// New creates a new Import.
func New() sdk.Import {
	return &framework.Import{
		Root: &root{},
	}
}

// NewObject creates a static object for a given Go value. This may
// return a RuntimeObj with a Value that is a gobridge.Import. Therefore,
// this function should only be used with the runtime evaluator.
func NewObject(raw interface{}) (object.Object, error) {
	v := reflectReturn(raw)
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
	// If we have Gob data (tests), then decode it. We use gob as a
	// transfer layer so that we can transfer more complex structs.
	if data, ok := raw[teststructs.GobKey]; ok {
		dataBytes, err := hex.DecodeString(data.(string))
		if err != nil {
			return fmt.Errorf("error decoding test data: %s", err)
		}

		raw = map[string]interface{}{}
		if err := gob.NewDecoder(bytes.NewReader(dataBytes)).Decode(&raw); err != nil {
			return fmt.Errorf("error decoding test data: %s", err)
		}
	}

	// Set our root namespace
	m.Namespace = &mapNS{objects: raw}

	return nil
}
