// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package raftutil

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// getAvailableSpace returns the available disk space in bytes for the given path.
func getAvailableSpace(path string) (uint64, error) {
	// Get the absolute path to ensure we have a valid path.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Extract the volume root (e.g., "C:\\" from "C:\\path\\to\\dir").
	volumePath := filepath.VolumeName(absPath)
	if volumePath == "" {
		return 0, fmt.Errorf("failed to determine volume for path: %s", absPath)
	}

	// Ensure the volume path ends with a backslash for the Windows API.
	if volumePath[len(volumePath)-1] != '\\' {
		volumePath += "\\"
	}

	// Convert to UTF-16 for Windows API.
	volumePathPtr, err := syscall.UTF16PtrFromString(volumePath)
	if err != nil {
		return 0, fmt.Errorf("failed to convert path to UTF-16: %w", err)
	}

	var freeBytesAvailable uint64
	var totalNumberOfBytes uint64
	var totalNumberOfFreeBytes uint64

	// Call GetDiskFreeSpaceExW.
	ret, _, err := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(volumePathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)

	if ret == 0 {
		// The syscall returned an error.
		if err != nil && err != syscall.Errno(0) {
			return 0, fmt.Errorf("failed to get disk space info: %w", err)
		}
		return 0, fmt.Errorf("failed to get disk space info: unknown error")
	}

	return freeBytesAvailable, nil
}
