// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd
// +build !linux,!darwin,!freebsd,!netbsd,!openbsd

package qemu

const (
	// Don't enforce any path limit.
	maxSocketPathLen = 0
)
