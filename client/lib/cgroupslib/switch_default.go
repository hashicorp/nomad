// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package cgroupslib

// GetMode returns OFF on non-Linux systems.
func GetMode() Mode {
	return OFF
}
