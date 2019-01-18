package docker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsParentPath(t *testing.T) {
	require.True(t, isParentPath("/a/b/c", "/a/b/c"))
	require.True(t, isParentPath("/a/b/c", "/a/b/c/d"))
	require.True(t, isParentPath("/a/b/c", "/a/b/c/d/e"))

	require.False(t, isParentPath("/a/b/c", "/a/b/d"))
	require.False(t, isParentPath("/a/b/c", "/a/b/cd"))
	require.False(t, isParentPath("/a/b/c", "/a/d/c"))
	require.False(t, isParentPath("/a/b/c", "/d/e/c"))
}
