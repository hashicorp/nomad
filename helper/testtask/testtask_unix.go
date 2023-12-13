// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package testtask

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

func executeProcessGroup(gid string) {
	// pgrp <group_int> puts the pid in a new process group
	grp, err := strconv.Atoi(gid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert process group number %q: %v\n", gid, err)
		os.Exit(1)
	}
	if err := syscall.Setpgid(0, grp); err != nil {
		fmt.Fprintf(os.Stderr, "failed to set process group: %v\n", err)
		os.Exit(1)
	}
}
