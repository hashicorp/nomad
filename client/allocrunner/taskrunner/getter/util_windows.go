//go:build windows

package getter

import (
	"os"
	"path/filepath"
	"syscall"
)

// attributes returns the system process attributes to run
// the sandbox process with
func attributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func credentials() (uint32, uint32) {
	return 0, 0
}

// lockdown has no effect on windows
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
