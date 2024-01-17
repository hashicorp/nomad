// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build unix

package utils

import (
	"runtime"

	"golang.org/x/sys/unix"
)

// IsLinuxOS returns true if the operating system is some Linux distribution.
func IsLinuxOS() bool {
	return runtime.GOOS == "linux"
}

// IsUnixRoot returns true if system is unix and user running is effectively root
func IsUnixRoot() bool {
	return unix.Geteuid() == 0
}
