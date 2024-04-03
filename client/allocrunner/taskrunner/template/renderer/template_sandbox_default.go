// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// sandbox is the non-Windows sandbox implementation, which relies on chroot.
// Although chroot is not an appropriate boundary for tasks (implicitly
// untrusted), here the only code that's executing is Nomad itself. Returns the
// new destPath inside the chroot.
func sandbox(sandboxPath, destPath string) (string, error) {

	err := syscall.Chroot(sandboxPath)
	if err != nil {
		// if the user is running in unsupported non-root configuration, we
		// can't build the sandbox, but need to handle this gracefully
		fmt.Fprintf(os.Stderr, "template-render sandbox %q not available: %v",
			sandboxPath, err)
		return destPath, nil
	}

	destPath, err = filepath.Rel(sandboxPath, destPath)
	if err != nil {
		return "", fmt.Errorf("could not find destination path relative to chroot: %w", err)
	}
	if !filepath.IsAbs(destPath) {
		destPath = "/" + destPath
	}

	return destPath, nil
}
