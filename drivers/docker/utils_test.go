// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
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

func TestParseDockerImage(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		Image string
		Repo  string
		Tag   string
	}{
		{"host:5000/library/hello-world", "host:5000/library/hello-world", "latest"},
		{"host:5000/library/hello-world:1.0", "host:5000/library/hello-world", "1.0"},
		{"library/hello-world:1.0", "library/hello-world", "1.0"},
		{"library/hello-world", "library/hello-world", "latest"},
		{"library/hello-world:latest", "library/hello-world", "latest"},
		{"library/hello-world@sha256:f5233545e43561214ca4891fd1157e1c3c563316ed8e237750d59bde73361e77", "library/hello-world@sha256:f5233545e43561214ca4891fd1157e1c3c563316ed8e237750d59bde73361e77", ""},
		{"my-registry:9090/hello-world@sha256:c7e3309ebb8805855bc1ccc24d24588748710e43925b39e563bd5541cbcbad91", "my-registry:9090/hello-world@sha256:c7e3309ebb8805855bc1ccc24d24588748710e43925b39e563bd5541cbcbad91", ""},
		{"my-registry:9090/hello-world:my-tag@sha256:c7e3309ebb8805855bc1ccc24d24588748710e43925b39e563bd5541cbcbad91", "my-registry:9090/hello-world@sha256:c7e3309ebb8805855bc1ccc24d24588748710e43925b39e563bd5541cbcbad91", ""},
	}
	for _, test := range tests {
		t.Run(test.Image, func(t *testing.T) {
			repo, tag := parseDockerImage(test.Image)
			print("repo", repo)
			must.Eq(t, test.Repo, repo)
			must.Eq(t, test.Tag, tag)
		})
	}
}
