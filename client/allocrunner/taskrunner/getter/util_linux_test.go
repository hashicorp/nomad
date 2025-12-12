// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package getter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/go-homedir"
	"github.com/shoenig/go-landlock"
	"github.com/shoenig/test/must"
)

func TestUtil_loadVersionControlGlobalConfigs(t *testing.T) {
	// not parallel

	const filePerm = 0o644
	const dirPerm = 0o755

	fakeEtc := t.TempDir()
	fakeHome := t.TempDir()
	fakeDev := t.TempDir()

	homedir.DisableCache = true
	t.Cleanup(func() {
		homedir.DisableCache = false
	})

	t.Setenv("HOME", fakeHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(fakeHome, ".config"))

	const (
		homeSSH          = ".ssh"
		homeKnownHosts   = ".ssh/known_hosts"
		gitGlobalFile    = ".gitconfig"
		gitGlobalFileXDG = "git/config"
	)

	var (
		gitSystem      = filepath.Join(fakeEtc, "gitconfig")
		gitGlobal      = filepath.Join(fakeHome, ".gitconfig")
		gitGlobalXDG   = filepath.Join(fakeHome, ".config/git/config")
		hgFile         = filepath.Join(fakeEtc, "hgrc")
		hgDir          = filepath.Join(fakeEtc, "hgrc.d")
		etcPasswd      = filepath.Join(fakeEtc, "passwd")
		etcKnownHosts  = filepath.Join(fakeEtc, "ssh/ssh_known_hosts")
		sshDir         = filepath.Join(fakeHome, homeSSH)
		knownHostsFile = filepath.Join(fakeHome, homeKnownHosts)
		urandom        = filepath.Join(fakeDev, "urandom")
	)

	err := os.WriteFile(gitSystem, []byte("git"), filePerm)
	must.NoError(t, err)

	err = os.WriteFile(gitGlobal, []byte("git"), filePerm)
	must.NoError(t, err)

	must.NoError(t, os.MkdirAll(filepath.Dir(gitGlobalXDG), 0755))
	err = os.WriteFile(gitGlobalXDG, []byte("git"), filePerm)
	must.NoError(t, err)

	err = os.WriteFile(hgFile, []byte("hg"), filePerm)
	must.NoError(t, err)

	err = os.Mkdir(hgDir, dirPerm)
	must.NoError(t, err)

	err = os.WriteFile(etcPasswd, []byte("etc passwd"), filePerm)
	must.NoError(t, err)

	err = os.Mkdir(filepath.Join(fakeEtc, "ssh"), dirPerm)
	must.NoError(t, err)

	err = os.WriteFile(etcKnownHosts, []byte("etc known hosts"), filePerm)
	must.NoError(t, err)

	err = os.Mkdir(sshDir, dirPerm)
	must.NoError(t, err)

	err = os.WriteFile(knownHostsFile, []byte("home known hosts"), filePerm)
	must.NoError(t, err)

	err = os.WriteFile(urandom, []byte("urandom"), filePerm)
	must.NoError(t, err)

	paths := filesForVCS(
		homeSSH,
		homeKnownHosts,
		etcPasswd,
		etcKnownHosts,
		gitSystem,
		gitGlobalFile,
		gitGlobalFileXDG,
		hgFile,
		hgDir,
		urandom,
	)
	must.SliceEqual(t, []*landlock.Path{
		landlock.Dir(sshDir, "r"),
		landlock.File(knownHostsFile, "rw"),
		landlock.File(etcPasswd, "r"),
		landlock.File(etcKnownHosts, "r"),
		landlock.File(gitSystem, "r"),
		landlock.File(gitGlobal, "r"),
		landlock.File(gitGlobalXDG, "r"),
		landlock.File(hgFile, "r"),
		landlock.Dir(hgDir, "r"),
		landlock.File(urandom, "r"),
	}, paths)
}
