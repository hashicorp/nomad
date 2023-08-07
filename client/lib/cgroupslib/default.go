// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package cgroupslib

// LinuxResourcesPath does nothing on non-Linux systems
func LinuxResourcesPath(string, string) string {
	return ""
}

// MaybeDisableMemorySwappiness does nothing on non-Linux systems
func MaybeDisableMemorySwappiness() *int {
	return nil
}
