// Package teststructs contains structs used for tests only.
package teststructs

import (
	"encoding/gob"
)

func init() {
	gob.Register(map[string]interface{}{})
	gob.Register(StructEmbeddedSingle{})
	gob.Register(StructEmbeddedSingleUnexportedValue{})
	gob.Register(StructEmptyTag{})
	gob.Register(StructNamespace{})
	gob.Register(StructNestedSingle{})
	gob.Register(StructSquashedSingle{})
	gob.Register(StructSingle{})
	gob.Register(structSingle{})
	gob.Register(StructSlice{})
	gob.Register(StructSliceStructs{})
	gob.Register(StructSingleUnexportedValue{})
}

var GobKey = "_test_gob"

type StructNestedSingle struct{ Value StructSingle }
type structSingle struct{ Value int }
type StructSingle struct{ Value int }
type StructSlice struct{ Value []int }
type StructSliceStructs struct{ Value []StructSingle }

type StructEmbeddedSingle struct{ StructSingle }

type StructEmbeddedSingleUnexportedValue struct {
	structSingle
	Exported struct{}
}

type StructSquashedSingle struct {
	StructSingle `sentinel:",squash"`
}

type StructSingleUnexportedValue struct {
	value    int
	Exported struct{}
}

type StructEmptyTag struct {
	Value int `sentinel:""`
}

func StructSingleUnexported() *StructSingleUnexportedValue {
	return &StructSingleUnexportedValue{value: 30}
}

func StructEmbeddedSingleUnexported() *StructEmbeddedSingleUnexportedValue {
	return &StructEmbeddedSingleUnexportedValue{
		structSingle: structSingle{Value: 30},
	}
}

type StructNamespace struct {
	Static interface{}
}

func (ns StructNamespace) SentinelGet(key string) (interface{}, error) {
	switch key {
	case "value":
		return 42, nil

	case "dynamic":
		return ns, nil

	default:
		return nil, nil
	}
}
