// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build darwin

package docker

func setCPUSetCgroup(path string, pid int) error {
	return nil
}
