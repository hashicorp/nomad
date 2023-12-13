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
		ssh        = ".ssh"
		knownHosts = ".ssh/known_hosts"
	)

	var (
		gitConfig      = filepath.Join(fakeEtc, "gitconfig")
		hgFile         = filepath.Join(fakeEtc, "hgrc")
		hgDir          = filepath.Join(fakeEtc, "hgrc.d")
		etcPasswd      = filepath.Join(fakeEtc, "passwd")
		sshDir         = filepath.Join(fakeHome, ssh)
		knownHostsFile = filepath.Join(fakeHome, knownHosts)
	)

	err := os.WriteFile(gitConfig, []byte("git"), filePerm)
	must.NoError(t, err)

	err = os.WriteFile(hgFile, []byte("hg"), filePerm)
	must.NoError(t, err)

	err = os.Mkdir(hgDir, dirPerm)
	must.NoError(t, err)

	err = os.WriteFile(etcPasswd, []byte("x:y:z"), filePerm)
	must.NoError(t, err)

	err = os.Mkdir(sshDir, dirPerm)
	must.NoError(t, err)

	err = os.WriteFile(knownHostsFile, []byte("abc123"), filePerm)
	must.NoError(t, err)

	paths := filesForVCS(ssh, knownHosts, etcPasswd, gitConfig, hgFile, hgDir)
	must.SliceEqual(t, []*landlock.Path{
		landlock.Dir(sshDir, "r"),
		landlock.File(knownHostsFile, "rw"),
		landlock.File(etcPasswd, "r"),
		landlock.File(gitConfig, "r"),
		landlock.File(hgFile, "r"),
		landlock.Dir(hgDir, "r"),
	}, paths)
}
