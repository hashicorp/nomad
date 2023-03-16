//go:build windows

package getter

import (
	"os"
	"path/filepath"
	"syscall"
)

// attributes is not implemented on Windows
func attributes() *syscall.SysProcAttr {
	return nil
}

// credentials is not implemented on Windows
func credentials() (uint32, uint32) {
	return 0, 0
}

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
