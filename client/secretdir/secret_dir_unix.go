// +build dragonfly freebsd linux netbsd openbsd solaris

package secretdir

import (
	"fmt"
	"os"
	"syscall"
)

// create creates a tmpfs folder at the secret dir path
func (s *SecretDir) create(sizeMB int64) error {
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return err
	}

	var flags uintptr
	flags = syscall.MS_NOEXEC
	options := fmt.Sprintf("size=%dm", sizeMB)
	err := syscall.Mount("tmpfs", s.Dir, "tmpfs", flags, options)
	return os.NewSyscallError("mount", err)
}

// destroy unmounts the tmpfs folder and deletes it
func (s *SecretDir) destroy() error {
	if err := syscall.Unmount(s.Dir, 0); err != nil {
		return err
	}

	return os.RemoveAll(s.Dir)
}
