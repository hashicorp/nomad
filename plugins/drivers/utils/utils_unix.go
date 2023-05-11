// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package utils

import (
	"golang.org/x/sys/unix"
)

// IsUnixRoot returns true if system is unix and user running is effectively root
func IsUnixRoot() bool {
	return unix.Geteuid() == 0
}
