// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package users

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/shoenig/test/must"
	"golang.org/x/sys/unix"
)

func TestLookup_Linux(t *testing.T) {
	cases := []struct {
		username string

		expErr  error
		expUser *user.User
	}{
		{username: "nobody", expUser: &user.User{Username: "nobody", Uid: "65534", Gid: "65534", Name: "nobody", HomeDir: "/nonexistent"}}, // ubuntu
		{username: "root", expUser: &user.User{Username: "root", Uid: "0", Gid: "0", Name: "root", HomeDir: "/root"}},
		{username: "doesnotexist", expErr: errors.New("user: unknown user doesnotexist")},
	}

	for _, tc := range cases {
		t.Run(tc.username, func(t *testing.T) {
			u, err := Lookup(tc.username)
			if tc.expErr != nil {
				must.EqError(t, tc.expErr, err.Error())
			} else {
				must.Eq(t, tc.expUser, u)
			}
		})
	}
}

func TestWriteFileFor_Linux(t *testing.T) {
	// This is really how you have to retrieve umask. See `man 2 umask`
	umask := unix.Umask(0)
	unix.Umask(umask)

	secretDir := t.TempDir()
	path := filepath.Join(secretDir, "secret.txt")
	contents := []byte("TOO MANY SECRETS")

	must.NoError(t, WriteFileFor(secretDir, "secret.txt", contents, "nobody"))

	stat, err := os.Lstat(path)
	must.NoError(t, err)
	must.True(t, stat.Mode().IsRegular(),
		must.Sprintf("expected %s to be a regular file but found %#o", path, stat.Mode()))

	linuxStat, ok := stat.Sys().(*syscall.Stat_t)
	must.True(t, ok, must.Sprintf("expected stat.Sys() to be a *syscall.Stat_t but found %T", stat.Sys()))

	current, err := Current()
	must.NoError(t, err)

	if current.Username == "root" {
		t.Logf("Running as root: asserting %s is owned by nobody", path)
		nobody, err := Lookup("nobody")
		must.NoError(t, err)
		must.Eq(t, nobody.Uid, fmt.Sprintf("%d", linuxStat.Uid))
		must.Eq(t, 0o600&(^umask), int(stat.Mode()))
	} else {
		t.Logf("Running as non-root: asserting %s is world readable", path)
		must.Eq(t, current.Uid, fmt.Sprintf("%d", linuxStat.Uid))
		must.Eq(t, 0o666&(^umask), int(stat.Mode()))
	}
}

func TestLookupUnix_root(t *testing.T) {
	uid, gid, home, err := LookupUnix("root")
	must.NoError(t, err)
	must.Zero(t, uid)         // linux
	must.Zero(t, gid)         // linux
	must.Eq(t, "/root", home) // ubuntu specific
}

func TestLookupUnix_nobody(t *testing.T) {
	uid, gid, home, err := LookupUnix("nobody")
	must.NoError(t, err)
	must.Eq(t, 65534, uid)           // systemd specific
	must.Eq(t, 65534, gid)           // systemd specific
	must.Eq(t, "/nonexistent", home) // ubuntu specific
}
