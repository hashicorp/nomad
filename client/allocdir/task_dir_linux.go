// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"fmt"
	"os"
	"path/filepath"

	multierror "github.com/hashicorp/go-multierror"
)

// unmountSpecialDirs unmounts the dev and proc file system from the chroot. No
// error is returned if the directories do not exist or have already been
// unmounted.
func (t *TaskDir) unmountSpecialDirs() error {
	errs := new(multierror.Error)
	dev := filepath.Join(t.Dir, "dev")
	if pathExists(dev) {
		if err := unlinkDir(dev); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to unmount dev %q: %v", dev, err))
		} else if err := os.RemoveAll(dev); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to delete dev directory %q: %v", dev, err))
		}
	}

	// Unmount proc.
	proc := filepath.Join(t.Dir, "proc")
	if pathExists(proc) {
		if err := unlinkDir(proc); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to unmount proc %q: %v", proc, err))
		} else if err := os.RemoveAll(proc); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to delete proc directory %q: %v", dev, err))
		}
	}

	return errs.ErrorOrNil()
}
