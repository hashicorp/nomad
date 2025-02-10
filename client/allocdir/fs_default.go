// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package allocdir

import "os"

// mountDir bind mounts old to next using the given file mode.
func mountDir(old, next string, uid, gid int, mode os.FileMode) error {
	panic("not implemented")
}
