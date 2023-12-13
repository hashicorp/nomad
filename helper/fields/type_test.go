// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fields

import "testing"

func TestFieldTypeString(t *testing.T) {
	if s := TypeString.String(); s != "string" {
		t.Fatalf("bad: expected 'string' got: %s", s)
	}

	if s := TypeInt.String(); s != "integer" {
		t.Fatalf("bad: expected 'integer' got: %s", s)
	}

	if s := TypeBool.String(); s != "boolean" {
		t.Fatalf("bad: expected 'boolean' got: %s", s)
	}

	if s := TypeMap.String(); s != "map" {
		t.Fatalf("bad: expected 'map' got: %v", s)
	}

	if s := TypeArray.String(); s != "array" {
		t.Fatalf("bad: expected 'array' got: %v", s)
	}

	if s := TypeInvalid.String(); s != "unknown type" {
		t.Fatalf("bad: expected 'unknown type' got: %v", s)
	}
}

func TestFieldTypeZero(t *testing.T) {
	if z := TypeString.Zero(); z != "" {
		t.Fatalf("bad: expected \"\" got: %v", z)
	}

	if z := TypeInt.Zero(); z != 0 {
		t.Fatalf("bad: expected 0 got: %v", z)
	}

	if z := TypeBool.Zero(); z != false {
		t.Fatalf("bad: expected false got: %v", z)
	}

	z := TypeMap.Zero()
	if _, ok := z.(map[string]interface{}); !ok {
		t.Fatalf("bad: expected map[string]interface{} got: %v", z)
	}

	z = TypeArray.Zero()
	if _, ok := z.([]interface{}); !ok {
		t.Fatalf("bad: expected []interface{} got: %v", z)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	TypeInvalid.Zero()
}
