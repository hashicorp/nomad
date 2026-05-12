// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build linux || freebsd || netbsd || openbsd

package fifo

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func mkfifo(path string, mode uint32) (err error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("error opening fifo parent directory %q: %v", dir, err)
	}
	defer root.Close()

	parent, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("error getting file handle to fifo parent directory %q: %v", dir, err)
	}
	defer parent.Close()

	// os.Root doesn't support creating a FIFO, so we need to drop to the
	// syscall and grab the parent's FD
	err = unix.Mkfifoat(int(parent.Fd()), base, mode)
	if err != nil {
		return fmt.Errorf("error creating fifo: %w", err)
	}
	return nil
}
