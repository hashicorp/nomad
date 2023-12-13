// Copyright (c) HashiCorp, Inc.
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

	"github.com/hashicorp/nomad/helper/testlog"
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

	path := filepath.Join(t.TempDir(), "secret.txt")
	contents := []byte("TOO MANY SECRETS")

	must.NoError(t, WriteFileFor(path, contents, "nobody"))

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

// TestSocketFileFor_Linux asserts that when running as root on Linux socket
// files are created with least permissions. If running as non-root then we
// leave the socket file as world writable.
func TestSocketFileFor_Linux(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.sock")

	logger := testlog.HCLogger(t)
	ln, err := SocketFileFor(logger, path, "nobody")
	must.NoError(t, err)
	must.NotNil(t, ln)
	t.Cleanup(func() {
		_ = ln.Close()
	})

	stat, err := os.Lstat(path)
	must.NoError(t, err)
	must.False(t, stat.Mode().IsRegular(),
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
		must.Eq(t, 0o600, int(stat.Mode().Perm()))
	} else {
		t.Logf("Running as non-root: asserting %s is world writable", path)
		must.Eq(t, current.Uid, fmt.Sprintf("%d", linuxStat.Uid))
		must.Eq(t, 0o666, int(stat.Mode().Perm()))
	}
}
