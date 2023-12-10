// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lang

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestWalkMap(t *testing.T) {
	m := map[int]string{
		1: "one",
		3: "three",
		4: "four",
		2: "two",
		5: "five",
	}

	result := make([]string, 0, 5)

	f := func(_ int, v string) bool {
		result = append(result, v)
		return true
	}

	WalkMap(m, f)

	must.Eq(t, []string{"one", "two", "three", "four", "five"}, result)
}

func TestWalkMap_halt(t *testing.T) {
	m := map[int]string{
		5: "five",
		1: "one",
		3: "three",
		4: "four",
	}

	result := make([]string, 0, 3)

	f := func(k int, v string) bool {
		if k%2 == 0 {
			// halt if we find an even key
			return false
		}
		result = append(result, v)
		return true
	}

	WalkMap(m, f)

	must.Eq(t, []string{"one", "three"}, result)
}
