// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux && !freebsd && !netbsd && !openbsd && !windows

package fifo

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func mkfifo(path string, mode uint32) (err error) {
	// macOS doesn't support mkfifoat
	err = unix.Mkfifo(path, mode)
	if err != nil {
		return fmt.Errorf("error creating fifo: %w", err)
	}
	return nil
}
