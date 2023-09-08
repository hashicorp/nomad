// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux && !windows

package getter

import (
	"path/filepath"
)

// lockdown is not implemented by default
func lockdown(string, string) error {
	return nil
}

// defaultEnvironment is the default minimal environment variables for Unix-like
// operating systems.
func defaultEnvironment(taskDir string) map[string]string {
	tmpDir := filepath.Join(taskDir, "tmp")
	return map[string]string{
		"PATH":   "/usr/local/bin:/usr/bin:/bin",
		"TMPDIR": tmpDir,
	}
}
