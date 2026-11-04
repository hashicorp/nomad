// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package clientsockets

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/shoenig/test/must"
)

// TestSocketFileFor_Linux asserts that when running as root on Linux socket
// files are created with least permissions. If running as non-root then we
// leave the socket file as world writable.
func TestSocketFileFor_Linux(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.sock")

	logger := testlog.HCLogger(t)
	ln, err := SocketFileFor(logger, path, "nobody", false)
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

	current, err := users.Current()
	must.NoError(t, err)

	if current.Username == "root" {
		t.Logf("Running as root: asserting %s is owned by nobody", path)
		nobody, err := users.Lookup("nobody")
		must.NoError(t, err)
		must.Eq(t, nobody.Uid, fmt.Sprintf("%d", linuxStat.Uid))
		must.Eq(t, 0o600, int(stat.Mode().Perm()))
	} else {
		t.Logf("Running as non-root: asserting %s is world writable", path)
		must.Eq(t, current.Uid, fmt.Sprintf("%d", linuxStat.Uid))
		must.Eq(t, 0o666, int(stat.Mode().Perm()))
	}
}
