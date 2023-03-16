//go:build linux

package getter

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/mitchellh/go-homedir"
	"github.com/shoenig/go-landlock"
	"golang.org/x/sys/unix"
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
// will run as - for now this is always the same user the Nomad agent is
// running as.
func credentials() (uint32, uint32) {
	return userUID, userGID
}

// findHomeDir returns the home directory as provided by os.UserHomeDir. In case
// os.UserHomeDir returns an error, we return /root if the current process is being
// run by root, or /dev/null otherwise.
func findHomeDir() string {
	// When running as a systemd unit the process may not have the $HOME
	// environment variable set, and os.UserHomeDir will return an error.

	// if home is set, just use that
	if home, err := homedir.Dir(); err == nil {
		return home
	}

	// if we are the root user, use "/root"
	if unix.Getuid() == 0 {
		return "/root"
	}

	// nothing safe to do
	return "/nonexistent"
}

// defaultEnvironment is the default minimal environment variables for Linux.
func defaultEnvironment(taskDir string) map[string]string {
	tmpDir := filepath.Join(taskDir, "tmp")
	homeDir := findHomeDir()
	return map[string]string{
		"PATH":   "/usr/local/bin:/usr/bin:/bin",
		"TMPDIR": tmpDir,
		"HOME":   homeDir,
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

	paths = append(paths, additionalFilesForVCS()...)
	locker := landlock.New(paths...)
	return locker.Lock(landlock.Mandatory)
}

func additionalFilesForVCS() []*landlock.Path {
	const (
		sshDir        = ".ssh"                  // git ssh
		knownHosts    = ".ssh/known_hosts"      // git ssh
		etcPasswd     = "/etc/passwd"           // git ssh
		gitGlobalFile = "/etc/gitconfig"        // https://git-scm.com/docs/git-config#SCOPES
		hgGlobalFile  = "/etc/mercurial/hgrc"   // https://www.mercurial-scm.org/doc/hgrc.5.html#files
		hgGlobalDir   = "/etc/mercurial/hgrc.d" // https://www.mercurial-scm.org/doc/hgrc.5.html#files
	)
	return filesForVCS(
		sshDir,
		knownHosts,
		etcPasswd,
		gitGlobalFile,
		hgGlobalFile,
		hgGlobalDir,
	)
}

func filesForVCS(
	sshDir,
	knownHosts,
	etcPasswd,
	gitGlobalFile,
	hgGlobalFile,
	hgGlobalDir string) []*landlock.Path {

	// omit ssh if there is no home directory
	home := findHomeDir()
	sshDir = filepath.Join(home, sshDir)
	knownHosts = filepath.Join(home, knownHosts)

	// only add if a path exists
	exists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}

	result := make([]*landlock.Path, 0, 6)
	if exists(sshDir) {
		result = append(result, landlock.Dir(sshDir, "r"))
	}
	if exists(knownHosts) {
		result = append(result, landlock.File(knownHosts, "rw"))
	}
	if exists(etcPasswd) {
		result = append(result, landlock.File(etcPasswd, "r"))
	}
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
