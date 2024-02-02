// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package shim

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_split(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		paths []string
		cmds  []string
	}{
		{
			name:  "env",
			args:  []string{"--", "env"},
			paths: nil,
			cmds:  []string{"env"},
		},
		{
			name:  "cat",
			args:  []string{"/etc/passwd:r", "--", "cat", "/etc/passwd"},
			paths: []string{"/etc/passwd:r"},
			cmds:  []string{"cat", "/etc/passwd"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paths, cmds := split(tc.args)
			must.Eq(t, tc.paths, paths)
			must.Eq(t, tc.cmds, cmds)
		})
	}
}
