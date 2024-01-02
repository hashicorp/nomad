// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux
// +build linux

package host

import (
	"bufio"
	"os"
	"strings"
)

// mountedPaths produces a list of mounts
func mountedPaths() []string {
	fh, err := os.Open("/proc/mounts")
	if err != nil {
		return []string{err.Error()}
	}
	rd := bufio.NewReader(fh)

	var paths []string
	for {
		str, err := rd.ReadString('\n')
		if err != nil {
			break
		}

		ln := strings.Split(str, " ")
		switch ln[2] {
		case "autofs", "binfmt_misc", "cgroup", "debugfs",
			"devpts", "devtmpfs",
			"fusectl", "fuse.lxcfs",
			"hugetlbfs", "mqueue",
			"proc", "pstore", "rpc_pipefs", "securityfs",
			"sysfs", "tmpfs", "vboxsf":
			continue
		default:
		}

		paths = append(paths, ln[1])
	}

	return paths
}
