// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux && !windows
// +build !linux,!windows

package fingerprint

// linkSpeed returns the default link speed
func (f *NetworkFingerprint) linkSpeed(device string) int {
	return 0
}
