// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fsisolation

// Mode is an enum to describe what kind of filesystem isolation a
// driver supports.
type Mode string

const (
	// IsolationNone means no isolation. The host filesystem is used.
	None = Mode("none")

	// IsolationChroot means the driver will use a chroot on the host
	// filesystem.
	Chroot = Mode("chroot")

	// IsolationImage means the driver uses an image.
	Image = Mode("image")

	// IsolationUnveil means the driver and client will work together using
	// unveil() syscall semantics (i.e. landlock on linux) isolate the host
	// filesytem from workloads.
	Unveil = Mode("unveil")
)
