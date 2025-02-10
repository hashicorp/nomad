// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package cgroupslib

// PathCG1 returns empty string on non-Linux systems
func PathCG1(allocID, taskName, iface string) string {
	return ""
}

// CustomPathCG1 returns empty string on non-Linux systems
func CustomPathCG1(controller, path string) string {
	return ""
}

// CustomPathCG2 returns empty string on non-Linux systems
func CustomPathCG2(path string) string {
	return ""
}
