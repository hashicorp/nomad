// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package testutils

func (*DriverHarness) MakeTaskCgroup(string, string) {
	// nothing
}
