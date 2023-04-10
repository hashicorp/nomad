// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux
// +build linux

package qemu

const (
	// https://man7.org/linux/man-pages/man7/unix.7.html
	maxSocketPathLen = 108
)
