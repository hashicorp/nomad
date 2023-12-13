// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package docker

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandPath(t *testing.T) {
	cases := []struct {
		base     string
		target   string
		expected string
	}{
		{"/tmp/alloc/task", ".", "/tmp/alloc/task"},
		{"/tmp/alloc/task", "..", "/tmp/alloc"},

		{"/tmp/alloc/task", "d1/d2", "/tmp/alloc/task/d1/d2"},
		{"/tmp/alloc/task", "../d1/d2", "/tmp/alloc/d1/d2"},
		{"/tmp/alloc/task", "../../d1/d2", "/tmp/d1/d2"},

		{"/tmp/alloc/task", "c:/home/user", "c:/home/user"},
		{"/tmp/alloc/task", "c:/home/user/..", "c:/home"},

		{"/tmp/alloc/task", `//./pipe/named_pipe`, `//./pipe/named_pipe`},
	}

	for _, c := range cases {
		t.Run(c.expected, func(t *testing.T) {
			require.Equal(t, c.expected, filepath.ToSlash(expandPath(c.base, c.target)))
		})
	}
}

func TestParseVolumeSpec_Windows(t *testing.T) {
	validCases := []struct {
		name          string
		bindSpec      string
		hostPath      string
		containerPath string
		mode          string
	}{
		{
			"basic mount",
			`c:\windows:e:\containerpath`,
			`c:\windows`,
			`e:\containerpath`,
			"",
		},
		{
			"relative path",
			`relativepath:e:\containerpath`,
			`relativepath`,
			`e:\containerpath`,
			"",
		},
		{
			"named pipe",
			`//./pipe/named_pipe://./pipe/named_pipe`,
			`\\.\pipe\named_pipe`,
			`//./pipe/named_pipe`,
			"",
		},
	}

	for _, c := range validCases {
		t.Run("valid:"+c.name, func(t *testing.T) {
			hp, cp, m, err := parseVolumeSpec(c.bindSpec, "windows")
			require.NoError(t, err)
			require.Equal(t, c.hostPath, hp)
			require.Equal(t, c.containerPath, cp)
			require.Equal(t, c.mode, m)
		})
	}

	invalidCases := []string{
		// linux path
		"/linux-path",
		// windows single path entry
		`e:\containerpath`,
	}

	for _, c := range invalidCases {
		t.Run("invalid:"+c, func(t *testing.T) {
			hp, cp, m, err := parseVolumeSpec(c, "windows")
			require.Errorf(t, err, "expected error but parsed as %s:%s:%s", hp, cp, m)
		})
	}
}
