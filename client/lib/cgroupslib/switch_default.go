// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package cgroupslib

// GetMode returns OFF on non-Linux systems.
func GetMode() Mode {
	return OFF
}
