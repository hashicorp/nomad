// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows
// +build !windows

package allocdir

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
	"github.com/shoenig/test/must"
)

// Test that building a chroot will skip nonexistent directories.
func TestTaskDir_EmbedNonexistent(t *testing.T) {
	ci.Parallel(t)

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, tmp, "test")
	defer d.Destroy()
	td := d.NewTaskDir(t1)
	must.NoError(t, d.Build())

	fakeDir := "/foobarbaz"
	mapping := map[string]string{fakeDir: fakeDir}
	must.NoError(t, td.embedDirs(mapping))
}

// Test that building a chroot copies files from the host into the task dir.
func TestTaskDir_EmbedDirs(t *testing.T) {
	ci.Parallel(t)

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, tmp, "test")
	defer d.Destroy()
	td := d.NewTaskDir(t1)
	must.NoError(t, d.Build())

	// Create a fake host directory, with a file, and a subfolder that contains
	// a file.
	host := t.TempDir()

	subDirName := "subdir"
	subDir := filepath.Join(host, subDirName)
	must.NoError(t, os.MkdirAll(subDir, 0o777))

	file := "foo"
	subFile := "bar"
	must.NoError(t, os.WriteFile(filepath.Join(host, file), []byte{'a'}, 0o777))
	must.NoError(t, os.WriteFile(filepath.Join(subDir, subFile), []byte{'a'}, 0o777))

	// Create mapping from host dir to task dir.
	taskDest := "bin/test/"
	mapping := map[string]string{host: taskDest}
	must.NoError(t, td.embedDirs(mapping))

	exp := []string{filepath.Join(td.Dir, taskDest, file), filepath.Join(td.Dir, taskDest, subDirName, subFile)}
	for _, f := range exp {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Fatalf("File %v not embedded: %v", f, err)
		}
	}
}

// Test that task dirs for image based isolation don't require root.
func TestTaskDir_NonRoot_Image(t *testing.T) {
	requireNonRoot(t)

	ci.Parallel(t)

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, tmp, "test")
	defer d.Destroy()
	td := d.NewTaskDir(t1)
	must.NoError(t, d.Build())
	must.NoError(t, td.Build(fsisolation.Image, nil, "nobody"))
}

// Test that task dirs with no isolation don't require root.
func TestTaskDir_NonRoot(t *testing.T) {
	requireNonRoot(t)

	ci.Parallel(t)

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, tmp, "test")
	defer d.Destroy()
	td := d.NewTaskDir(t1)
	must.NoError(t, d.Build())
	must.NoError(t, td.Build(fsisolation.None, nil, "nobody"))

	// ${TASK_DIR}/alloc should not exist!
	if _, err := os.Stat(td.SharedTaskDir); !os.IsNotExist(err) {
		t.Fatalf("Expected a NotExist error for shared alloc dir in task dir: %q", td.SharedTaskDir)
	}
}

func TestTaskDir_NonRoot_Unveil(t *testing.T) {
	requireNonRoot(t)

	ci.Parallel(t)

	tmp := t.TempDir()

	// non-root, should still work for tasks running as the same user as the
	// nomad client agent
	u, err := user.Current()
	must.NoError(t, err)

	d := NewAllocDir(testlog.HCLogger(t), tmp, tmp, "test")
	defer d.Destroy()
	td := d.NewTaskDir(t1)
	must.NoError(t, d.Build())
	must.NoError(t, td.Build(fsisolation.Unveil, nil, u.Username))
	fi, err := os.Stat(td.MountsTaskDir)
	must.NoError(t, err)
	must.NotNil(t, fi)
}

func TestTaskDir_Root_Unveil(t *testing.T) {
	requireRoot(t)

	ci.Parallel(t)

	tmp := t.TempDir()

	// root, can build task dirs for another user
	d := NewAllocDir(testlog.HCLogger(t), tmp, tmp, "test")
	defer d.Destroy()
	td := d.NewTaskDir(t1)
	must.NoError(t, d.Build())
	must.NoError(t, td.Build(fsisolation.Unveil, nil, "nobody"))
	fi, err := os.Stat(td.MountsTaskDir)
	must.NoError(t, err)
	must.NotNil(t, fi)
}
