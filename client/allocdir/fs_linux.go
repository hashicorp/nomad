// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

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

	return syscall.Mount(src, dst, "", syscall.MS_BIND, "")
}

// unlinkDir unmounts a bind mounted directory as Linux doesn't support
// hardlinking directories. If the dir is already unmounted no error is
// returned.
func unlinkDir(dir string) error {
	if err := syscall.Unmount(dir, 0); err != nil {
		if err != syscall.EINVAL {
			return err
		}
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

		flags := uintptr(syscall.MS_NOEXEC)
		options := fmt.Sprintf("size=%dm", secretDirTmpfsSize)
		if err := syscall.Mount("tmpfs", dir, "tmpfs", flags, options); err != nil {
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
			if err != syscall.ENOENT {
				return os.NewSyscallError("unmount", err)
			}
		}

	}
	return os.RemoveAll(dir)
}
