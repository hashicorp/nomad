//go:build windows

package getter

import (
	"fmt"
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
func lockdown(string) error {
	return nil
}

func minimalVars(taskDir string) []string {
	tmpDir := filepath.Join(taskDir, "tmp")
	return []string{
		fmt.Sprintf("HOMEPATH=%s", os.Getenv("HOMEPATH")),
		fmt.Sprintf("HOMEDRIVE=%s", os.Getenv("HOMEDRIVE")),
		fmt.Sprintf("USERPROFILE=%s", os.Getenv("USERPROFILE")),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		fmt.Sprintf("TMP=%s", tmpDir),
		fmt.Sprintf("TEMP=%s", tmpDir),
	}
}
