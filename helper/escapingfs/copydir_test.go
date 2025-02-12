// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package escapingfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/shoenig/test/must"
	"golang.org/x/sys/unix"
)

func TestCopyDir(t *testing.T) {

	testDir := t.TempDir()
	src := filepath.Join(testDir, "src")
	dst := filepath.Join(testDir, "dst")

	must.NoError(t, os.Mkdir(src, 0700))
	must.NoError(t, os.WriteFile(filepath.Join(src, "foo"), []byte("foo"), 0770))
	must.NoError(t, os.WriteFile(filepath.Join(src, "bar"), []byte("bar"), 0555))
	must.NoError(t, os.Mkdir(filepath.Join(src, "bazDir"), 0700))
	must.NoError(t, os.WriteFile(filepath.Join(src, "bazDir", "baz"), []byte("baz"), 0555))

	err := CopyDir(src, dst)
	must.NoError(t, err)

	// This is really how you have to retrieve umask. See `man 2 umask`
	umask := unix.Umask(0)
	unix.Umask(umask)

	must.FileContains(t, filepath.Join(dst, "foo"), "foo")
	must.FileMode(t, filepath.Join(dst, "foo"), fs.FileMode(0o770&(^umask)))
	must.FileContains(t, filepath.Join(dst, "bar"), "bar")
	must.FileMode(t, filepath.Join(dst, "bar"), fs.FileMode(0o555&(^umask)))
	must.FileContains(t, filepath.Join(dst, "bazDir", "baz"), "baz")
	must.FileMode(t, filepath.Join(dst, "bazDir", "baz"), fs.FileMode(0o555&(^umask)))
}
