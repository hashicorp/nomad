package allocdir

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const (
	// secretDirTmpfsSize is the size of the tmpfs per task in MBs
	secretDirTmpfsSize = 1

	// secretMarker is the filename of the marker created so Nomad doesn't
	// try to mount the secrets tmpfs more than once
	secretMarker = ".nomad-mount"
)

// linkDir bind mounts src to dst as Linux doesn't support hardlinking
// directories.
func linkDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0777); err != nil {
		return err
	}

	return unix.Mount(src, dst, "", unix.MS_BIND, "")
}

// unlinkDir unmounts a bind mounted directory as Linux doesn't support
// hardlinking directories. If the dir is already unmounted no error is
// returned.
func unlinkDir(dir string) error {
	return unmount(dir)
}

// bindMount mounts src to dst with support for read only mounting.
// Assumes that filepath.Dir(dst) exists already
func bindMount(src, dst string, readOnly bool) error {
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		// check if destination is available and create a corresponding type if it is not present
		stats, err := os.Stat(src)
		if err != nil {
			return err
		}

		if stats.IsDir() {
			if err := os.Mkdir(dst, 0777); err != nil {
				return err
			}
		} else {
			if _, err := os.Create(dst); err != nil {
				return err
			}

		}
	}

	err := unix.Mount(src, dst, "", unix.MS_BIND, "")
	if err != nil || !readOnly {
		return err
	}

	// mount yet again for read-only flag
	return unix.Mount("", dst, "", unix.MS_BIND|unix.MS_RDONLY|unix.MS_REMOUNT, "")
}

// unmount a bind mount.  If the target is already unmounted, no error is returned
func unmount(path string) error {
	if err := unix.Unmount(path, 0); err != nil && err != unix.EINVAL {
		return err
	}
	return nil

}

// createSecretDir creates the secrets dir folder at the given path using a
// tmpfs
func createSecretDir(dir string) error {
	// Only mount the tmpfs if we are root
	if unix.Geteuid() == 0 {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return err
		}

		// Check for marker file and skip mounting if it exists
		marker := filepath.Join(dir, secretMarker)
		if _, err := os.Stat(marker); err == nil {
			return nil
		}

		var flags uintptr
		flags = unix.MS_NOEXEC
		options := fmt.Sprintf("size=%dm", secretDirTmpfsSize)
		if err := unix.Mount("tmpfs", dir, "tmpfs", flags, options); err != nil {
			return os.NewSyscallError("mount", err)
		}

		// Create the marker file so we don't try to mount more than once
		f, err := os.OpenFile(marker, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			// Hard fail since if this fails something is really wrong
			return err
		}
		f.Close()
		return nil
	}

	return os.MkdirAll(dir, 0777)
}

// createSecretDir removes the secrets dir folder
func removeSecretDir(dir string) error {
	if unix.Geteuid() == 0 {
		if err := unlinkDir(dir); err != nil {
			// Ignore invalid path errors
			if err != unix.ENOENT {
				return os.NewSyscallError("unmount", err)
			}
		}

	}
	return os.RemoveAll(dir)
}
