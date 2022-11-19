//go:build linux

package getter

import (
	"fmt"
	"path/filepath"

	"github.com/hashicorp/nomad/helper/users"
	"github.com/shoenig/go-landlock"
)

var (
	// version of landlock available, 0 otherwise
	version int
)

func init() {
	v, err := landlock.Detect()
	if err == nil {
		version = v
	}
}

// credentials returns the UID and GID of the user the child process
// will run as. On unix systems this will be the nobody user if available.
func credentials() (uint32, uint32) {
	uid, gid := users.NobodyIDs()
	return uid, gid
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

// lockdown isolates this process to only be able to write and
// create files in the task's task directory.
// dir - the task directory
// executes - indicates use of git or hg
//
// Only applies to Linux, when useable.
func lockdown(dir string, executes bool) error {
	// landlock not present in the kernel, do not sandbox
	if !landlock.Available() {
		return nil
	}

	// can only landlock git with version 2+, otherwise skip
	if executes && version < 2 {
		return nil
	}

	paths := []*landlock.Path{
		landlock.DNS(),
		landlock.Certs(),
		landlock.Shared(),
		landlock.Dir("/bin", "rx"),
		landlock.Dir("/usr/bin", "rx"),
		landlock.Dir("/usr/local/bin", "rx"),
		landlock.Dir(dir, "rwc"),
	}
	locker := landlock.New(paths...)
	return locker.Lock(landlock.Mandatory)
}
