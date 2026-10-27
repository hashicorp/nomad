// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package resolvconf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/lib/resolvconf"
	"github.com/shoenig/test/must"
)

func Test_copySystemDNS(t *testing.T) {
	data, err := os.ReadFile(resolvconf.Path())
	must.NoError(t, err)

	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	must.NoError(t, err)
	t.Cleanup(func() { root.Close() })

	must.NoError(t, copySystemDNS(root))
	must.FileExists(t, filepath.Join(dir, "resolv.conf"))

	tmpResolv, readErr := os.ReadFile(filepath.Join(dir, "resolv.conf"))
	must.NoError(t, readErr)
	must.Eq(t, data, tmpResolv)
}
