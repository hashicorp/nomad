package cty

import (
	"bytes"
	"testing"

	"encoding/gob"
)

func TestGobabilty(t *testing.T) {
	tests := []Value{
		StringVal("hi"),
		True,
		NumberIntVal(1),
		NumberFloatVal(96.5),
		ListVal([]Value{True}),
		MapVal(map[string]Value{"true": True}),
		SetVal([]Value{True}),
		TupleVal([]Value{True}),
		ObjectVal(map[string]Value{"true": True}),
	}

	for _, testValue := range tests {
		t.Run(testValue.GoString(), func(t *testing.T) {
			tv := testGob{
				testValue,
			}

			buf := &bytes.Buffer{}
			enc := gob.NewEncoder(buf)

			err := enc.Encode(tv)
			if err != nil {
				t.Fatalf("gob encode error: %s", err)
			}

			var ov testGob

			dec := gob.NewDecoder(buf)
			err = dec.Decode(&ov)
			if err != nil {
				t.Fatalf("gob decode error: %s", err)
			}

			if !ov.Value.RawEquals(tv.Value) {
				t.Errorf("value did not survive gobbing\ninput:  %#v\noutput: %#v", tv, ov)
			}
		})
	}
}

type testGob struct {
	Value Value
}
