//go:build !linux && !windows

package getter

import (
	"fmt"
	"path/filepath"
	"syscall"
)

// credentials returns root ids outside of Linux
func credentials() (uint32, uint32) {
	uid := syscall.Getuid()
	gid := syscall.Getgid()
	return uint32(uid), uint32(gid)
}

// minimalVars returns the minimal environment set for artifact
// downloader sandbox
func minimalVars(taskDir string) []string {
	tmpDir := filepath.Join(taskDir, "tmp")
	return []string{
		fmt.Sprintf("PATH=/usr/local/bin:/usr/bin:/bin"),
		fmt.Sprintf("TMPDIR=%s", tmpDir),
	}
}

// lockdown applies only to Linux
func lockdown(string, bool) error {
	return nil
}
