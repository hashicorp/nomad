//go:build linux

package getter

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/hashicorp/nomad/helper/users"
	"github.com/shoenig/go-landlock"
)

var (
	// userUID is the current user's uid
	userUID uint32

	// userGID is the current user's gid
	userGID uint32
)

func init() {
	userUID = uint32(syscall.Getuid())
	userGID = uint32(syscall.Getgid())
}

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

// credentials returns the UID and GID of the user the child process
// will run as. On Linux systems this will be the nobody user if Nomad
// is being run as the root user, or the user Nomad is being run as
// otherwise.
func credentials() (uint32, uint32) {
	switch userUID {
	case 0:
		return users.NobodyIDs()
	default:
		return userUID, userGID
	}
}

// defaultEnvironment is the default minimal environment variables for Linux.
func defaultEnvironment(taskDir string) map[string]string {
	tmpDir := filepath.Join(taskDir, "tmp")
	return map[string]string{
		"PATH":   "/usr/local/bin:/usr/bin:/bin",
		"TMPDIR": tmpDir,
	}
}

// lockdown isolates this process to only be able to write and
// create files in the task's task directory.
// dir - the task directory
//
// Only applies to Linux, when available.
func lockdown(allocDir, taskDir string) error {
	// landlock not present in the kernel, do not sandbox
	if !landlock.Available() {
		return nil
	}
	paths := []*landlock.Path{
		landlock.DNS(),
		landlock.Certs(),
		landlock.Shared(),
		landlock.Dir("/bin", "rx"),
		landlock.Dir("/usr/bin", "rx"),
		landlock.Dir("/usr/local/bin", "rx"),
		landlock.Dir(allocDir, "rwc"),
		landlock.Dir(taskDir, "rwc"),
	}
	paths = append(paths, systemVersionControlGlobalConfigs()...)
	locker := landlock.New(paths...)
	return locker.Lock(landlock.Mandatory)
}

func systemVersionControlGlobalConfigs() []*landlock.Path {
	const (
		gitGlobalFile = "/etc/gitconfig"        // https://git-scm.com/docs/git-config#SCOPES
		hgGlobalFile  = "/etc/mercurial/hgrc"   // https://www.mercurial-scm.org/doc/hgrc.5.html#files
		hgGlobalDir   = "/etc/mercurial/hgrc.d" // https://www.mercurial-scm.org/doc/hgrc.5.html#files
	)
	return loadVersionControlGlobalConfigs(gitGlobalFile, hgGlobalFile, hgGlobalDir)
}

func loadVersionControlGlobalConfigs(gitGlobalFile, hgGlobalFile, hgGlobalDir string) []*landlock.Path {
	exists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}
	result := make([]*landlock.Path, 0, 3)
	if exists(gitGlobalFile) {
		result = append(result, landlock.File(gitGlobalFile, "r"))
	}
	if exists(hgGlobalFile) {
		result = append(result, landlock.File(hgGlobalFile, "r"))
	}
	if exists(hgGlobalDir) {
		result = append(result, landlock.Dir(hgGlobalDir, "r"))
	}
	return result
}
