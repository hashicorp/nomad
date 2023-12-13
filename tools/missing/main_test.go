// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_isCoveredOne(t *testing.T) {
	try := func(p string, exp bool) {
		result := isCoveredOne(p, "foo/bar")
		must.Eq(t, exp, result)
	}
	try("baz", false)
	try("foo", false)
	try("foo/bar/baz", false)
	try("foo/bar", true)
	try("foo/bar/...", true)
	try("foo/...", true)
	try("abc/...", false)
}
