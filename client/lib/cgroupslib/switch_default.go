// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package cgroupslib

// GetMode returns OFF on non-Linux systems.
func GetMode() Mode {
	return OFF
}
