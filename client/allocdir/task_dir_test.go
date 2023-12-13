// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
)

// Test that building a chroot will skip nonexistent directories.
func TestTaskDir_EmbedNonexistent(t *testing.T) {
	ci.Parallel(t)

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, "test")
	defer d.Destroy()
	td := d.NewTaskDir(t1.Name)
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	fakeDir := "/foobarbaz"
	mapping := map[string]string{fakeDir: fakeDir}
	if err := td.embedDirs(mapping); err != nil {
		t.Fatalf("embedDirs(%v) should should skip %v since it does not exist", mapping, fakeDir)
	}
}

// Test that building a chroot copies files from the host into the task dir.
func TestTaskDir_EmbedDirs(t *testing.T) {
	ci.Parallel(t)

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, "test")
	defer d.Destroy()
	td := d.NewTaskDir(t1.Name)
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Create a fake host directory, with a file, and a subfolder that contains
	// a file.
	host := t.TempDir()

	subDirName := "subdir"
	subDir := filepath.Join(host, subDirName)
	if err := os.MkdirAll(subDir, 0777); err != nil {
		t.Fatalf("Failed to make subdir %v: %v", subDir, err)
	}

	file := "foo"
	subFile := "bar"
	if err := os.WriteFile(filepath.Join(host, file), []byte{'a'}, 0777); err != nil {
		t.Fatalf("Couldn't create file in host dir %v: %v", host, err)
	}

	if err := os.WriteFile(filepath.Join(subDir, subFile), []byte{'a'}, 0777); err != nil {
		t.Fatalf("Couldn't create file in host subdir %v: %v", subDir, err)
	}

	// Create mapping from host dir to task dir.
	taskDest := "bin/test/"
	mapping := map[string]string{host: taskDest}
	if err := td.embedDirs(mapping); err != nil {
		t.Fatalf("embedDirs(%v) failed: %v", mapping, err)
	}

	exp := []string{filepath.Join(td.Dir, taskDest, file), filepath.Join(td.Dir, taskDest, subDirName, subFile)}
	for _, f := range exp {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Fatalf("File %v not embedded: %v", f, err)
		}
	}
}

// Test that task dirs for image based isolation don't require root.
func TestTaskDir_NonRoot_Image(t *testing.T) {
	ci.Parallel(t)
	if os.Geteuid() == 0 {
		t.Skip("test should be run as non-root user")
	}
	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, "test")
	defer d.Destroy()
	td := d.NewTaskDir(t1.Name)
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	if err := td.Build(false, nil); err != nil {
		t.Fatalf("TaskDir.Build failed: %v", err)
	}
}

// Test that task dirs with no isolation don't require root.
func TestTaskDir_NonRoot(t *testing.T) {
	ci.Parallel(t)
	if os.Geteuid() == 0 {
		t.Skip("test should be run as non-root user")
	}

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, "test")
	defer d.Destroy()
	td := d.NewTaskDir(t1.Name)
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	if err := td.Build(false, nil); err != nil {
		t.Fatalf("TaskDir.Build failed: %v", err)
	}

	// ${TASK_DIR}/alloc should not exist!
	if _, err := os.Stat(td.SharedTaskDir); !os.IsNotExist(err) {
		t.Fatalf("Expected a NotExist error for shared alloc dir in task dir: %q", td.SharedTaskDir)
	}
}
