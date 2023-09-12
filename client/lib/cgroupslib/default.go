// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package cgroupslib

// LinuxResourcesPath does nothing on non-Linux systems
func LinuxResourcesPath(string, string, bool) string {
	return ""
}

// MaybeDisableMemorySwappiness does nothing on non-Linux systems
func MaybeDisableMemorySwappiness() *uint64 {
	return nil
}
