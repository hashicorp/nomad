// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package getter

import (
	"os"
	"path/filepath"
)

// lockdown is not implemented on Windows
func lockdown(string, string) error {
	return nil
}

// defaultEnvironment is the default minimal environment variables for Windows.
func defaultEnvironment(taskDir string) map[string]string {
	tmpDir := filepath.Join(taskDir, "tmp")
	return map[string]string{
		"HOMEPATH":    os.Getenv("HOMEPATH"),
		"HOMEDRIVE":   os.Getenv("HOMEDRIVE"),
		"USERPROFILE": os.Getenv("USERPROFILE"),
		"PATH":        os.Getenv("PATH"),
		"TMP":         tmpDir,
		"TEMP":        tmpDir,
	}
}
