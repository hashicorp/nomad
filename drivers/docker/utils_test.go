package docker

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCgroupPermission(t *testing.T) {
	positiveCases := []string{
		"r",
		"rw",
		"rwm",
		"mr",
		"mrw",
		"",
	}

	for _, c := range positiveCases {
		t.Run("positive case: "+c, func(t *testing.T) {
			require.True(t, validateCgroupPermission(c))
		})
	}

	negativeCases := []string{
		"q",
		"asdf",
		"rq",
	}

	for _, c := range negativeCases {
		t.Run("negative case: "+c, func(t *testing.T) {
			require.False(t, validateCgroupPermission(c))
		})
	}

}

func TestExpandPath(t *testing.T) {
	cases := []struct {
		base     string
		target   string
		expected string
	}{
		{"/tmp/alloc/task", "/home/user", "/home/user"},
		{"/tmp/alloc/task", "/home/user/..", "/home"},

		{"/tmp/alloc/task", ".", "/tmp/alloc/task"},
		{"/tmp/alloc/task", "..", "/tmp/alloc"},

		{"/tmp/alloc/task", "d1/d2", "/tmp/alloc/task/d1/d2"},
		{"/tmp/alloc/task", "../d1/d2", "/tmp/alloc/d1/d2"},
		{"/tmp/alloc/task", "../../d1/d2", "/tmp/d1/d2"},
	}

	for _, c := range cases {
		t.Run(c.expected, func(t *testing.T) {
			require.Equal(t, c.expected, filepath.ToSlash(expandPath(c.base, c.target)))
		})
	}
}

func TestIsParentPath(t *testing.T) {
	require.True(t, isParentPath("/a/b/c", "/a/b/c"))
	require.True(t, isParentPath("/a/b/c", "/a/b/c/d"))
	require.True(t, isParentPath("/a/b/c", "/a/b/c/d/e"))

	require.False(t, isParentPath("/a/b/c", "/a/b/d"))
	require.False(t, isParentPath("/a/b/c", "/a/b/cd"))
	require.False(t, isParentPath("/a/b/c", "/a/d/c"))
	require.False(t, isParentPath("/a/b/c", "/d/e/c"))
}
