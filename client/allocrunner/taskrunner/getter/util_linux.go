// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package getter

import (
	"errors"
	"os"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"
	"github.com/mitchellh/go-homedir"
	"github.com/shoenig/go-landlock"
	"golang.org/x/sys/unix"
)

// initialDirs are the initial set of paths configured for landlock
var initialDirs = map[string]string{
	"/bin":           "rx",
	"/usr/bin":       "rx",
	"/usr/local/bin": "rx",
	"/usr/libexec":   "rx",
}

// findHomeDir returns the home directory as provided by homedir.Dir(). In case
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
	return "/dev/null"
}

// findConfigDir returns the config directory as provided by os.UserConfigDir. In
// case os.UserConfigDir returns an error, the path is built if possible. Otherwise
// a nonexistant path is returned.
func findConfigDir() string {
	config, err := os.UserConfigDir()
	if err == nil {
		return config
	}

	return filepath.Join(findHomeDir(), ".config")
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

// lockdownAvailable returns if lockdown is implemented for
// the current platform.
func lockdownAvailable() bool {
	return landlock.Available()
}

// lockdown isolates this process to only be able to write and
// create files in the task's task directory.
// dir - the task directory
//
// Only applies to Linux, when available.
func lockdown(l log.Logger, allocDir, taskDir string, extra []string) error {
	// landlock not present in the kernel, do not sandbox
	if !landlock.Available() {
		return nil
	}
	paths := []*landlock.Path{
		landlock.DNS(),
		landlock.Certs(),
		landlock.Shared(),
		landlock.Dir(allocDir, "rwc"),
		landlock.Dir(taskDir, "rwc"),
	}

	// Add the initial directories
	for p, mode := range initialDirs {
		_, err := os.Stat(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// paths that do not exist are skipped
				l.Debug("landlock is skipping path that does not exist", "path", p)
			} else {
				// other errors should be logged to provide context on why
				// the path is not included
				l.Warn("landlock setup failed to stat path, skipping", "path", p, "error", err)
			}

			continue
		}
		paths = append(paths, landlock.Dir(p, mode))
	}

	for _, p := range extra {
		path, err := landlock.ParsePath(p)
		if err != nil {
			return err
		}
		paths = append(paths, path)
	}
	paths = append(paths, additionalFilesForVCS()...)
	locker := landlock.New(paths...)
	return locker.Lock(landlock.Mandatory)
}

func additionalFilesForVCS() []*landlock.Path {
	const (
		homeSSHDir       = ".ssh"                     // git ssh
		homeKnownHosts   = ".ssh/known_hosts"         // git ssh
		etcPasswd        = "/etc/passwd"              // git ssh
		etcKnownHosts    = "/etc/ssh/ssh_known_hosts" // git ssh
		gitSystemFile    = "/etc/gitconfig"           // https://git-scm.com/docs/git-config#SCOPES
		gitGlobalFile    = ".gitconfig"               // https://git-scm.com/docs/git-config#SCOPES
		gitGlobalFileXDG = "git/config"               // https://git-scm.com/docs/git-config#SCOPES
		hgGlobalFile     = "/etc/mercurial/hgrc"      // https://www.mercurial-scm.org/doc/hgrc.5.html#files
		hgGlobalDir      = "/etc/mercurial/hgrc.d"    // https://www.mercurial-scm.org/doc/hgrc.5.html#files
		urandom          = "/dev/urandom"             // git
	)
	return filesForVCS(
		homeSSHDir,
		homeKnownHosts,
		etcPasswd,
		etcKnownHosts,
		gitSystemFile,
		gitGlobalFile,
		gitGlobalFileXDG,
		hgGlobalFile,
		hgGlobalDir,
		urandom,
	)
}

func filesForVCS(
	homeSSHDir,
	homeKnownHosts,
	etcPasswd,
	etcKnownHosts,
	gitSystemFile,
	gitGlobalFile,
	gitGlobalFileXDG,
	hgGlobalFile,
	hgGlobalDir,
	urandom string) []*landlock.Path {

	// omit ssh if there is no home directory
	home := findHomeDir()
	homeSSHDir = filepath.Join(home, homeSSHDir)
	homeKnownHosts = filepath.Join(home, homeKnownHosts)

	gitGlobalFile = filepath.Join(home, gitGlobalFile)
	gitGlobalFileXDG = filepath.Join(findConfigDir(), gitGlobalFileXDG)

	// detect if p exists
	exists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}

	result := make([]*landlock.Path, 0, 6)
	if exists(homeSSHDir) {
		result = append(result, landlock.Dir(homeSSHDir, "r"))
	}
	if exists(homeKnownHosts) {
		result = append(result, landlock.File(homeKnownHosts, "rw"))
	}
	if exists(etcPasswd) {
		result = append(result, landlock.File(etcPasswd, "r"))
	}
	if exists(etcKnownHosts) {
		result = append(result, landlock.File(etcKnownHosts, "r"))
	}
	if exists(gitSystemFile) {
		result = append(result, landlock.File(gitSystemFile, "r"))
	}
	if exists(gitGlobalFile) {
		result = append(result, landlock.File(gitGlobalFile, "r"))
	}
	if exists(gitGlobalFileXDG) {
		result = append(result, landlock.File(gitGlobalFileXDG, "r"))
	}
	if exists(hgGlobalFile) {
		result = append(result, landlock.File(hgGlobalFile, "r"))
	}
	if exists(hgGlobalDir) {
		result = append(result, landlock.Dir(hgGlobalDir, "r"))
	}
	if exists(urandom) {
		result = append(result, landlock.File(urandom, "r"))
	}

	return result
}
