package allocdir

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/hashicorp/go-multierror"
)

// mountSpecialDirs mounts the dev and proc file system from the host to the
// chroot
func (t *TaskDir) mountSpecialDirs() error {
	// Mount dev
	dev := filepath.Join(t.Dir, "dev")
	if !pathExists(dev) {
		if err := os.Mkdir(dev, 0777); err != nil {
			return fmt.Errorf("Mkdir(%v) failed: %v", dev, err)
		}
	}
	devEmpty, err := pathEmpty(dev)
	if err != nil {
		return fmt.Errorf("error listing %q: %v", dev, err)
	}
	if devEmpty {
		if err := syscall.Mount("none", dev, "devtmpfs", syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("Couldn't mount /dev to %v: %v", dev, err)
		}
	}

	// Mount proc
	proc := filepath.Join(t.Dir, "proc")
	if !pathExists(proc) {
		if err := os.Mkdir(proc, 0777); err != nil {
			return fmt.Errorf("Mkdir(%v) failed: %v", proc, err)
		}
	}
	procEmpty, err := pathEmpty(proc)
	if err != nil {
		return fmt.Errorf("error listing %q: %v", proc, err)
	}
	if procEmpty {
		if err := syscall.Mount("none", proc, "proc", syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("Couldn't mount /proc to %v: %v", proc, err)
		}
	}

	return nil
}

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
