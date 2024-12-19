// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package renderer

// sandbox is the Windows-specific sandbox implementation. Under Windows,
// symlinks can only be written by the Administrator (including the
// ContainerAdministrator user unfortunately used as the default for Docker). So
// our sandboxing is done by creating an AppContainer in the caller.
func sandbox(_, destPath string) (string, error) {
	return destPath, nil
}
