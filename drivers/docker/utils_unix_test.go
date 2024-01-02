// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package docker

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestValidateCgroupPermission(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

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

		{"/tmp/alloc/task", "/home/user", "/home/user"},
		{"/tmp/alloc/task", "/home/user/..", "/home"},
	}

	for _, c := range cases {
		t.Run(c.expected, func(t *testing.T) {
			require.Equal(t, c.expected, filepath.ToSlash(expandPath(c.base, c.target)))
		})
	}
}
