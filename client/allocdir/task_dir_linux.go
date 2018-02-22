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
		fstype := "devtmpfs"
		if err := syscall.Mount("none", dev, "devtmpfs", syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("Couldn't mount /dev to %v: %v", dev, err)
		}
		t.Mounts[dev] = fstype
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
		fstype := "proc"
		if err := syscall.Mount("none", proc, "proc", syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("Couldn't mount /proc to %v: %v", proc, err)
		}
		t.Mounts[proc] = fstype
	}

	return nil
}

// unmount unmounts all the mounts within task dir (special dirs and rbind mounts)
func (t *TaskDir) unmount() error {
	errs := new(multierror.Error)

	for mountPoint, mountType := range t.Mounts {
		if pathExists(mountPoint) {
			if err := unlinkDir(mountPoint); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("Failed to unmount %v (%v): %v", mountPoint, mountType, err))
			} else if err := os.RemoveAll(mountPoint); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("Failed to delete %v (%v): %v", mountPoint, mountType, err))
			}
		}
	}

	return errs.ErrorOrNil()
}

// bindDirs rbind-mounts files & dirs from the host to the chroot
func (t *TaskDir) bindDirs(entries map[string]string) error {
	for src, dst := range entries {
		// Check to see if file/directory exists on host.
		s, err := os.Stat(src)
		if os.IsNotExist(err) {
			continue
		}

		path := filepath.Join(t.Dir, dst)
		if !pathExists(path) {
			if s.IsDir() {
				if err := os.MkdirAll(path, s.Mode()); err != nil {
					return fmt.Errorf("Mkdir(%v) failed: %v", path, err)
				}
			} else {
				sourceDir := filepath.Dir(src)
				sourceDirStat, err := os.Stat(sourceDir)

				if err != nil {
					return fmt.Errorf("Stat(%v) failed: %v", sourceDir, err)
				}

				destDir := filepath.Dir(path)
				if !pathExists(destDir) {
					if err := os.MkdirAll(destDir, sourceDirStat.Mode()); err != nil {
						return fmt.Errorf("Mkdir(%v) failed: %v", destDir, err)
					}
				}
				newFile, err := os.Create(path)
				if err != nil {
					return fmt.Errorf("Create(%v) failed: %v", path, err)
				}
				newFile.Close()
			}

			// syscall.MS_PRIVATE - don't propagate unmount to the nested mounts
			// syscall.MS_BIND|syscall.MS_REC - include nested mounts
			if err := syscall.Mount(src, path, "", syscall.MS_PRIVATE|syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
				return fmt.Errorf("Couldn't mount %s to %v: %v", src, path, err)
			}
			t.Mounts[path] = src
		}
	}

	return nil
}
