// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import "testing"

// This file is intentionally empty to force early versions of Go
// to test compilation for tests.

func TestFail(t *testing.T) {
	t.Fatal("Boom")
}
