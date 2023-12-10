// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package uuid

import (
	"regexp"
	"testing"
)

func TestGenerate(t *testing.T) {
	prev := Generate()
	for i := 0; i < 100; i++ {
		id := Generate()
		if prev == id {
			t.Fatalf("Should get a new ID!")
		}

		matched, err := regexp.MatchString(
			"[\\da-f]{8}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{12}", id)
		if !matched || err != nil {
			t.Fatalf("expected match %s %v %s", id, matched, err)
		}
	}
}
