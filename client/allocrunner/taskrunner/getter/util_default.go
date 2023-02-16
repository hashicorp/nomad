//go:build !linux && !windows

package getter

import (
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

// defaultEnvironment is the default minimal environment variables for Unix-like
// operating systems.
func defaultEnvironment(taskDir string) map[string]string {
	tmpDir := filepath.Join(taskDir, "tmp")
	return map[string]string{
		"PATH":   "/usr/local/bin:/usr/bin:/bin",
		"TMPDIR": tmpDir,
	}
}

// lockdown applies only to Linux
func lockdown(string, string) error {
	return nil
}
