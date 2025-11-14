// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"path/filepath"
	"syscall"
)

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zstorage_windows.go storage_windows.go

//sys	getDiskSpaceEx(dirName *uint16, availableFreeBytes *uint64, totalBytes *uint64, totalFreeBytes *uint64) (err error) = kernel32.GetDiskFreeSpaceExW

// diskInfo inspects the filesystem for path and returns the volume name and
// the total bytes available on the file system.
func (f *StorageFingerprint) diskInfo(path string) (volume string, total uint64, err error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to determine absolute path for %s", path)
	}

	volume = filepath.VolumeName(absPath)

	absPathp, err := syscall.UTF16PtrFromString(absPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to convert \"%s\" to UTF16: %v", absPath, err)
	}

	if err := getDiskSpaceEx(absPathp, nil, &total, nil); err != nil {
		return "", 0, fmt.Errorf("failed to get free disk space for %s: %v", absPath, err)
	}

	return volume, total, nil
}
