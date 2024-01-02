// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package users

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/shoenig/test/must"
)

func TestLookup_Windows(t *testing.T) {
	stdlibUser, err := user.Current()
	must.NoError(t, err, must.Sprintf("error looking up current user using stdlib"))
	must.NotEq(t, "", stdlibUser.Username)

	helperUser, err := Current()
	must.NoError(t, err)

	must.Eq(t, stdlibUser.Username, helperUser.Username)

	lookupUser, err := Lookup(helperUser.Username)
	must.NoError(t, err)

	must.Eq(t, helperUser.Username, lookupUser.Username)
}

func TestLookup_Administrator(t *testing.T) {
	u, err := user.Lookup("Administrator")
	must.NoError(t, err)

	// Windows allows looking up unqualified names but will return a fully
	// qualified (eg prefixed with the local machine or domain)
	must.StrHasSuffix(t, "Administrator", u.Username)
}

func TestWriteFileFor_Windows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.txt")
	contents := []byte("TOO MANY SECRETS")

	must.NoError(t, WriteFileFor(path, contents, "Administrator"))
	stat, err := os.Lstat(path)
	must.NoError(t, err)
	must.True(t, stat.Mode().IsRegular(),
		must.Sprintf("expected %s to be a regular file but found %#o", path, stat.Mode()))

	// Assert Windows hits the fallback world-accessible case
	must.Eq(t, 0o666, stat.Mode().Perm())
}

// TestSocketFileFor_Windows asserts that socket files cannot be chowned on
// windows.
func TestSocketFileFor_Windows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.sock")

	ln, err := SocketFileFor(testlog.HCLogger(t), path, "Administrator")
	must.NoError(t, err)
	must.NotNil(t, ln)
	defer ln.Close()
	stat, err := os.Lstat(path)
	must.NoError(t, err)

	// Assert Windows hits the fallback world-accessible case
	must.Eq(t, 0o666, stat.Mode().Perm())
}
