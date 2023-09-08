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

	homedir.DisableCache = true
	t.Cleanup(func() {
		homedir.DisableCache = false
	})

	t.Setenv("HOME", fakeHome)

	const (
		homeSSH        = ".ssh"
		homeKnownHosts = ".ssh/known_hosts"
	)

	var (
		gitConfig      = filepath.Join(fakeEtc, "gitconfig")
		hgFile         = filepath.Join(fakeEtc, "hgrc")
		hgDir          = filepath.Join(fakeEtc, "hgrc.d")
		etcPasswd      = filepath.Join(fakeEtc, "passwd")
		etcKnownHosts  = filepath.Join(fakeEtc, "ssh/ssh_known_hosts")
		sshDir         = filepath.Join(fakeHome, homeSSH)
		knownHostsFile = filepath.Join(fakeHome, homeKnownHosts)
	)

	err := os.WriteFile(gitConfig, []byte("git"), filePerm)
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

	paths := filesForVCS(
		homeSSH,
		homeKnownHosts,
		etcPasswd,
		etcKnownHosts,
		gitConfig,
		hgFile,
		hgDir,
	)
	must.SliceEqual(t, []*landlock.Path{
		landlock.Dir(sshDir, "r"),
		landlock.File(knownHostsFile, "rw"),
		landlock.File(etcPasswd, "r"),
		landlock.File(etcKnownHosts, "r"),
		landlock.File(gitConfig, "r"),
		landlock.File(hgFile, "r"),
		landlock.Dir(hgDir, "r"),
	}, paths)
}
