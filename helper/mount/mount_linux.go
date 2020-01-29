// +build linux

package mount

import (
	docker_mount "github.com/docker/docker/pkg/mount"
)

// mounter provides the default implementation of mount.Mounter
// for the linux platform.
// Currently it delegates to the docker `mount` package.
type mounter struct {
}

// New returns a Mounter for the current system.
func New() Mounter {
	return &mounter{}
}

// IsNotAMountPoint determines if a directory is not a mountpoint.
// It does this by checking the path against the contents of /proc/self/mountinfo
func (m *mounter) IsNotAMountPoint(path string) (bool, error) {
	isMount, err := docker_mount.Mounted(path)
	return !isMount, err
}

func (m *mounter) Mount(device, target, mountType, options string) error {
	// Defer to the docker implementation of `Mount`, it's correct enough for our
	// usecase and avoids us needing to shell out to the `mount` utility.
	return docker_mount.Mount(device, target, mountType, options)
}
