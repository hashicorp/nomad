// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin

package docker

func setCPUSetCgroup(path string, pid int) error {
	return nil
}
