package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_isCoveredOne(t *testing.T) {
	try := func(p string, exp bool) {
		result := isCoveredOne(p, "foo/bar")
		require.Equal(t, exp, result)
	}
	try("baz", false)
	try("foo", false)
	try("foo/bar/baz", false)
	try("foo/bar", true)
	try("foo/bar/...", true)
	try("foo/...", true)
	try("abc/...", false)
}
