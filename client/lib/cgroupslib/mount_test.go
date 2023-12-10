// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"strings"
	"testing"

	"github.com/shoenig/test/must"
)

const (
	cg1 = `
33 29 0:27 / /run/lock rw,nosuid,nodev,noexec,relatime shared:6 - tmpfs tmpfs rw,size=5120k
34 25 0:28 / /sys/fs/cgroup ro,nosuid,nodev,noexec shared:9 - tmpfs tmpfs ro,mode=755
35 34 0:29 / /sys/fs/cgroup/unified rw,nosuid,nodev,noexec,relatime shared:10 - cgroup2 cgroup2 rw,nsdelegate
36 34 0:30 / /sys/fs/cgroup/systemd rw,nosuid,nodev,noexec,relatime shared:11 - cgroup cgroup rw,xattr,name=systemd
`

	cg2 = `
34 28 0:29 / /run/lock rw,nosuid,nodev,noexec,relatime shared:6 - tmpfs tmpfs rw,size=5120k,inode64
35 24 0:30 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:9 - cgroup2 cgroup2 rw,nsdelegate,memory_recursiveprot
36 24 0:31 / /sys/fs/pstore rw,nosuid,nodev,noexec,relatime shared:10 - pstore pstore rw
37 24 0:32 / /sys/firmware/efi/efivars rw,nosuid,nodev,noexec,relatime shared:11 - efivarfs efivarfs rw
`
)

func Test_scan(t *testing.T) {
	cases := []struct {
		name  string
		input string
		exp   Mode
	}{
		{
			name:  "v1",
			input: cg1,
			exp:   CG1,
		},
		{
			name:  "v2",
			input: cg2,
			exp:   CG2,
		},
		{
			name:  "empty",
			input: "",
			exp:   OFF,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := strings.NewReader(tc.input)
			result := scan(in)
			must.Eq(t, tc.exp, result)
		})
	}
}

func TestGetMode(t *testing.T) {
	mode := GetMode()
	ok := mode == CG1 || mode == CG2
	must.True(t, ok)
}
