//go:build !linux && !windows

package getter

import (
	"path/filepath"
	"syscall"
)

// attributes is not implemented by default
func attributes() *syscall.SysProcAttr {
	return nil
}

// credentials is not implemented by default
func credentials() (uint32, uint32) {
	return 0, 0
}

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
