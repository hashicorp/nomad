// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package testutils

func (*DriverHarness) MakeTaskCgroup(string, string) {
	// nothing
}
