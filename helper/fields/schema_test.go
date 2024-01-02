// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fields

import "testing"

func TestFieldSchemaDefaultOrZero(t *testing.T) {
	fs := &FieldSchema{
		Type:    TypeString,
		Default: "default",
	}

	if d := fs.DefaultOrZero(); d != "default" {
		t.Fatalf("bad: Expected: default Got: %s", d)
	}

	fs = &FieldSchema{
		Type: TypeString,
	}

	if d := fs.DefaultOrZero(); d != "" {
		t.Fatalf("bad: Expected: \"\" Got: %s", d)
	}
}
