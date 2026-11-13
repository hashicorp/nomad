// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package clientsockets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
	"github.com/shoenig/test/must"
)

// TestSocketFileFor_Windows asserts that socket files cannot be chowned on
// windows.
func TestSocketFileFor_Windows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.sock")

	ln, err := SocketFileFor(testlog.HCLogger(t), path, "Administrator", fsisolation.None)
	must.NoError(t, err)
	must.NotNil(t, ln)
	defer ln.Close()
	stat, err := os.Lstat(path)
	must.NoError(t, err)

	// Assert Windows hits the fallback world-accessible case
	must.Eq(t, 0o666, stat.Mode().Perm())
}
