// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package flock

import (
	"fmt"
	"os"
	"syscall"
)

func flock(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("%w: %q", ErrLocked, file.Name())
		}

		return fmt.Errorf("flock error: %w: %q", err, file.Name())
	}

	return nil
}

func funlock(file *os.File) error {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("funlock error: %w: %q", err, file.Name())
	}
	return nil
}
