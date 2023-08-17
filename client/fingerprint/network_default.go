// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux && !windows
// +build !linux,!windows

package fingerprint

// linkSpeed returns the default link speed
func (f *NetworkFingerprint) linkSpeed(device string) int {
	return 0
}
