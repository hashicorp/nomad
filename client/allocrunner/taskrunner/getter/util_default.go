//go:build !linux && !windows

package getter

import (
	"fmt"
	"path/filepath"
	"syscall"
)

// attributes returns the system process attributes to run
// the sandbox process with
func attributes() *syscall.SysProcAttr {
	uid, gid := credentials()
	return &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uid,
			Gid: gid,
		},
	}
}

// credentials returns the credentials of the user Nomad is running as
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
func lockdown(string) error {
	return nil
}
