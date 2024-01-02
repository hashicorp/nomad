// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin || freebsd || netbsd || openbsd
// +build darwin freebsd netbsd openbsd

package qemu

const (
	// https://man.openbsd.org/unix.4#ADDRESSING
	// https://www.freebsd.org/cgi/man.cgi?query=unix
	// https://github.com/apple/darwin-xnu/blob/main/bsd/man/man4/unix.4#L72
	// https://man.netbsd.org/unix.4#ADDRESSING
	maxSocketPathLen = 104
)
