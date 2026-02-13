// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package raftutil

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func getAvailableSpace(path string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("failed to get disk space info: %w", err)
	}

	// Bavail is the number of free blocks available to unprivileged users.
	// Multiply by block size to get bytes.
	availableSpace := stat.Bavail * uint64(stat.Bsize)
	return availableSpace, nil
}
