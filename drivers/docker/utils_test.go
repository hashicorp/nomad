// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestIsParentPath(t *testing.T) {
	ci.Parallel(t)
	require.True(t, isParentPath("/a/b/c", "/a/b/c"))
	require.True(t, isParentPath("/a/b/c", "/a/b/c/d"))
	require.True(t, isParentPath("/a/b/c", "/a/b/c/d/e"))

	require.False(t, isParentPath("/a/b/c", "/a/b/d"))
	require.False(t, isParentPath("/a/b/c", "/a/b/cd"))
	require.False(t, isParentPath("/a/b/c", "/a/d/c"))
	require.False(t, isParentPath("/a/b/c", "/d/e/c"))
}

func TestParseVolumeSpec_Linux(t *testing.T) {
	ci.Parallel(t)
	validCases := []struct {
		name          string
		bindSpec      string
		hostPath      string
		containerPath string
		mode          string
	}{
		{
			"absolute paths with mode",
			"/etc/host-path:/etc/container-path:rw",
			"/etc/host-path",
			"/etc/container-path",
			"rw",
		},
		{
			"absolute paths without mode",
			"/etc/host-path:/etc/container-path",
			"/etc/host-path",
			"/etc/container-path",
			"",
		},
		{
			"relative paths with mode",
			"etc/host-path:/etc/container-path:rw",
			"etc/host-path",
			"/etc/container-path",
			"rw",
		},
		{
			"relative paths without mode",
			"etc/host-path:/etc/container-path",
			"etc/host-path",
			"/etc/container-path",
			"",
		},
	}

	for _, c := range validCases {
		t.Run("valid:"+c.name, func(t *testing.T) {
			hp, cp, m, err := parseVolumeSpec(c.bindSpec, "linux")
			require.NoError(t, err)
			require.Equal(t, c.hostPath, hp)
			require.Equal(t, c.containerPath, cp)
			require.Equal(t, c.mode, m)
		})
	}

	invalidCases := []string{
		"/single-path",
	}

	for _, c := range invalidCases {
		t.Run("invalid:"+c, func(t *testing.T) {
			hp, cp, m, err := parseVolumeSpec(c, "linux")
			require.Errorf(t, err, "expected error but parsed as %s:%s:%s", hp, cp, m)
		})
	}
}
