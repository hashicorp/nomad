// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mount

// Mounter defines the set of methods to allow for mount operations on a system.
type Mounter interface {
	// IsNotAMountPoint detects if a provided directory is not a mountpoint.
	IsNotAMountPoint(file string) (bool, error)

	// Mount will mount filesystem according to the specified configuration, on
	// the condition that the target path is *not* already mounted. Options must
	// be specified like the mount or fstab unix commands: "opt1=val1,opt2=val2".
	Mount(device, target, mountType, options string) error
}

// Compile-time check to ensure all Mounter implementations satisfy
// the mount interface.
var _ Mounter = &mounter{}
