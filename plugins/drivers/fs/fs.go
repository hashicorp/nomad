// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fs

// Isolation is an enumeration to describe what kind of filesystem isolation
// a driver supports.
type Isolation string

const (
	// IsolationNone means no isolation. The host filesystem is used.
	IsolationNone = Isolation("none")

	// IsolationChroot means the driver will use a chroot on the host
	// filesystem.
	IsolationChroot = Isolation("chroot")

	// IsolationImage means the driver uses an image.
	IsolationImage = Isolation("image")

	// IsolationUnveil means the driver and client will work together using
	// unveil() syscall semantics (i.e. landlock on linux) isolate the host
	// filesytem from workloads.
	IsolationUnveil = Isolation("unveil")
)
