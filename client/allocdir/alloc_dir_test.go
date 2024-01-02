// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

var (
	t1 = &structs.Task{
		Name:   "web",
		Driver: "exec",
		Config: map[string]interface{}{
			"command": "/bin/date",
			"args":    "+%s",
		},
		Resources: &structs.Resources{
			DiskMB: 1,
		},
	}

	t2 = &structs.Task{
		Name:   "web2",
		Driver: "exec",
		Config: map[string]interface{}{
			"command": "/bin/date",
			"args":    "+%s",
		},
		Resources: &structs.Resources{
			DiskMB: 1,
		},
	}
)

// Test that AllocDir.Build builds just the alloc directory.
func TestAllocDir_BuildAlloc(t *testing.T) {
	ci.Parallel(t)

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, "test")
	defer d.Destroy()
	d.NewTaskDir(t1.Name)
	d.NewTaskDir(t2.Name)
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Check that the AllocDir and each of the task directories exist.
	if _, err := os.Stat(d.AllocDir); os.IsNotExist(err) {
		t.Fatalf("Build() didn't create AllocDir %v", d.AllocDir)
	}

	for _, task := range []*structs.Task{t1, t2} {
		tDir, ok := d.TaskDirs[task.Name]
		if !ok {
			t.Fatalf("Task directory not found for %v", task.Name)
		}

		if stat, _ := os.Stat(tDir.Dir); stat != nil {
			t.Fatalf("Build() created TaskDir %v", tDir.Dir)
		}

		if stat, _ := os.Stat(tDir.SecretsDir); stat != nil {
			t.Fatalf("Build() created secret dir %v", tDir.Dir)
		}
	}
}

// HACK: This function is copy/pasted from client.testutil to prevent a test
//
//	import cycle, due to testutil transitively importing allocdir. This
//	should be fixed after DriverManager is implemented.
func MountCompatible(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support mount")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("Must be root to run test")
	}
}

func TestAllocDir_MountSharedAlloc(t *testing.T) {
	ci.Parallel(t)
	MountCompatible(t)

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, "test")
	defer d.Destroy()
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Build 2 task dirs
	td1 := d.NewTaskDir(t1.Name)
	if err := td1.Build(true, nil); err != nil {
		t.Fatalf("error build task=%q dir: %v", t1.Name, err)
	}
	td2 := d.NewTaskDir(t2.Name)
	if err := td2.Build(true, nil); err != nil {
		t.Fatalf("error build task=%q dir: %v", t2.Name, err)
	}

	// Write a file to the shared dir.
	contents := []byte("foo")
	const filename = "bar"
	if err := os.WriteFile(filepath.Join(d.SharedDir, filename), contents, 0666); err != nil {
		t.Fatalf("Couldn't write file to shared directory: %v", err)
	}

	// Check that the file exists in the task directories
	for _, td := range []*TaskDir{td1, td2} {
		taskFile := filepath.Join(td.SharedTaskDir, filename)
		act, err := os.ReadFile(taskFile)
		if err != nil {
			t.Errorf("Failed to read shared alloc file from task dir: %v", err)
			continue
		}

		if !bytes.Equal(act, contents) {
			t.Errorf("Incorrect data read from task dir: want %v; got %v", contents, act)
		}
	}
}

func TestAllocDir_Snapshot(t *testing.T) {
	ci.Parallel(t)

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, "test")
	defer d.Destroy()
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Build 2 task dirs
	td1 := d.NewTaskDir(t1.Name)
	if err := td1.Build(false, nil); err != nil {
		t.Fatalf("error build task=%q dir: %v", t1.Name, err)
	}
	td2 := d.NewTaskDir(t2.Name)
	if err := td2.Build(false, nil); err != nil {
		t.Fatalf("error build task=%q dir: %v", t2.Name, err)
	}

	// Write a file to the shared dir.
	exp := []byte{'f', 'o', 'o'}
	file := "bar"
	if err := os.WriteFile(filepath.Join(d.SharedDir, "data", file), exp, 0666); err != nil {
		t.Fatalf("Couldn't write file to shared directory: %v", err)
	}

	// Write a symlink to the shared dir
	link := "qux"
	if err := os.Symlink("foo", filepath.Join(d.SharedDir, "data", link)); err != nil {
		t.Fatalf("Couldn't write symlink to shared directory: %v", err)
	}

	// Write a file to the task local
	exp = []byte{'b', 'a', 'r'}
	file1 := "lol"
	if err := os.WriteFile(filepath.Join(td1.LocalDir, file1), exp, 0666); err != nil {
		t.Fatalf("couldn't write file to task local directory: %v", err)
	}

	// Write a symlink to the task local
	link1 := "baz"
	if err := os.Symlink("bar", filepath.Join(td1.LocalDir, link1)); err != nil {
		t.Fatalf("couldn't write symlink to task local directory :%v", err)
	}

	var b bytes.Buffer
	if err := d.Snapshot(&b); err != nil {
		t.Fatalf("err: %v", err)
	}

	tr := tar.NewReader(&b)
	var files []string
	var links []string
	for {
		hdr, err := tr.Next()
		if err != nil && err != io.EOF {
			t.Fatalf("err: %v", err)
		}
		if err == io.EOF {
			break
		}
		if hdr.Typeflag == tar.TypeReg {
			files = append(files, hdr.FileInfo().Name())
		} else if hdr.Typeflag == tar.TypeSymlink {
			links = append(links, hdr.FileInfo().Name())
		}
	}

	if len(files) != 2 {
		t.Fatalf("bad files: %#v", files)
	}
	if len(links) != 2 {
		t.Fatalf("bad links: %#v", links)
	}
}

func TestAllocDir_Move(t *testing.T) {
	ci.Parallel(t)

	tmp1 := t.TempDir()
	tmp2 := t.TempDir()

	// Create two alloc dirs
	d1 := NewAllocDir(testlog.HCLogger(t), tmp1, "test")
	if err := d1.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	defer d1.Destroy()

	d2 := NewAllocDir(testlog.HCLogger(t), tmp2, "test")
	if err := d2.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	defer d2.Destroy()

	td1 := d1.NewTaskDir(t1.Name)
	if err := td1.Build(false, nil); err != nil {
		t.Fatalf("TaskDir.Build() faild: %v", err)
	}

	// Create but don't build second task dir to mimic alloc/task runner
	// behavior (AllocDir.Move() is called pre-TaskDir.Build).
	d2.NewTaskDir(t1.Name)

	dataDir := filepath.Join(d1.SharedDir, SharedDataDir)

	// Write a file to the shared dir.
	exp1 := []byte("foo")
	file1 := "bar"
	if err := os.WriteFile(filepath.Join(dataDir, file1), exp1, 0666); err != nil {
		t.Fatalf("Couldn't write file to shared directory: %v", err)
	}

	// Write a file to the task local
	exp2 := []byte("bar")
	file2 := "lol"
	if err := os.WriteFile(filepath.Join(td1.LocalDir, file2), exp2, 0666); err != nil {
		t.Fatalf("couldn't write to task local directory: %v", err)
	}

	// Move the d1 allocdir to d2
	if err := d2.Move(d1, []*structs.Task{t1}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure the files in d1 are present in d2
	fi, err := os.Stat(filepath.Join(d2.SharedDir, SharedDataDir, file1))
	if err != nil || fi == nil {
		t.Fatalf("data dir was not moved")
	}

	fi, err = os.Stat(filepath.Join(d2.TaskDirs[t1.Name].LocalDir, file2))
	if err != nil || fi == nil {
		t.Fatalf("task local dir was not moved")
	}
}

func TestAllocDir_EscapeChecking(t *testing.T) {
	ci.Parallel(t)

	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, "test")
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	defer d.Destroy()

	// Check that issuing calls that escape the alloc dir returns errors
	// List
	if _, err := d.List(".."); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("List of escaping path didn't error: %v", err)
	}

	// Stat
	if _, err := d.Stat("../foo"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("Stat of escaping path didn't error: %v", err)
	}

	// ReadAt
	if _, err := d.ReadAt("../foo", 0); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("ReadAt of escaping path didn't error: %v", err)
	}

	// BlockUntilExists
	if _, err := d.BlockUntilExists(context.Background(), "../foo"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("BlockUntilExists of escaping path didn't error: %v", err)
	}

	// ChangeEvents
	if _, err := d.ChangeEvents(context.Background(), "../foo", 0); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("ChangeEvents of escaping path didn't error: %v", err)
	}
}

// Test that `nomad fs` can't read secrets
func TestAllocDir_ReadAt_SecretDir(t *testing.T) {
	ci.Parallel(t)
	tmp := t.TempDir()

	d := NewAllocDir(testlog.HCLogger(t), tmp, "test")
	err := d.Build()
	require.NoError(t, err)
	defer func() {
		_ = d.Destroy()
	}()

	td := d.NewTaskDir(t1.Name)
	err = td.Build(false, nil)
	require.NoError(t, err)

	// something to write and test reading
	target := filepath.Join(t1.Name, TaskSecrets, "test_file")

	// create target file in the task secrets dir
	full := filepath.Join(d.AllocDir, target)
	err = os.WriteFile(full, []byte("hi"), 0600)
	require.NoError(t, err)

	// ReadAt of a file in the task secrets dir should fail
	_, err = d.ReadAt(target, 0)
	require.EqualError(t, err, "Reading secret file prohibited: web/secrets/test_file")
}

func TestAllocDir_SplitPath(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()

	dest := filepath.Join(dir, "/foo/bar/baz")
	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		t.Fatalf("err: %v", err)
	}

	info, err := splitPath(dest)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Testing that is 6 or more rather than 6 because on osx, the temp dir is
	// randomized.
	if len(info) < 6 {
		t.Fatalf("expected more than: %v, actual: %v", 6, len(info))
	}
}

func TestAllocDir_CreateDir(t *testing.T) {
	ci.Parallel(t)
	if syscall.Geteuid() != 0 {
		t.Skip("Must be root to run test")
	}

	dir := t.TempDir()

	// create a subdir and a file
	subdir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subdir, 0760); err != nil {
		t.Fatalf("err: %v", err)
	}
	subdirMode, err := os.Stat(subdir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create the above hierarchy under another destination
	dir1 := t.TempDir()

	if err := createDir(dir1, subdir); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure that the subdir had the right perm
	fi, err := os.Stat(filepath.Join(dir1, dir, "subdir"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fi.Mode() != subdirMode.Mode() {
		t.Fatalf("wrong file mode: %v, expected: %v", fi.Mode(), subdirMode.Mode())
	}
}

func TestPathFuncs(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()

	missingDir := filepath.Join(dir, "does-not-exist")

	if !pathExists(dir) {
		t.Errorf("%q exists", dir)
	}
	if pathExists(missingDir) {
		t.Errorf("%q does not exist", missingDir)
	}

	if empty, err := pathEmpty(dir); err != nil || !empty {
		t.Errorf("%q is empty and exists. empty=%v error=%v", dir, empty, err)
	}
	if empty, err := pathEmpty(missingDir); err == nil || empty {
		t.Errorf("%q is missing. empty=%v error=%v", missingDir, empty, err)
	}

	filename := filepath.Join(dir, "just-some-file")
	f, err := os.Create(filename)
	if err != nil {
		t.Fatalf("could not create %q: %v", filename, err)
	}
	f.Close()

	if empty, err := pathEmpty(dir); err != nil || empty {
		t.Errorf("%q is not empty. empty=%v error=%v", dir, empty, err)
	}
}

func TestAllocDir_DetectContentType(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	inputPath := "input/"
	var testFiles []string
	err := filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			testFiles = append(testFiles, path)
		}
		return err
	})
	require.Nil(err)

	expectedEncodings := map[string]string{
		"input/happy.gif": "image/gif",
		"input/image.png": "image/png",
		"input/nomad.jpg": "image/jpeg",
		"input/test.bin":  "application/octet-stream",
		"input/test.json": "application/json",
		"input/test.txt":  "text/plain; charset=utf-8",
		"input/test.go":   "text/plain; charset=utf-8",
		"input/test.hcl":  "text/plain; charset=utf-8",
	}
	for _, file := range testFiles {
		fileInfo, err := os.Stat(file)
		require.Nil(err)
		res := detectContentType(fileInfo, file)
		require.Equal(expectedEncodings[file], res, "unexpected output for %v", file)
	}
}

// TestAllocDir_SkipAllocDir asserts that building a chroot which contains
// itself will *not* infinitely recurse. AllocDirs should always skip embedding
// themselves into chroots.
//
// Warning: If this test fails it may fill your disk before failing, so be
// careful and/or confident.
func TestAllocDir_SkipAllocDir(t *testing.T) {
	ci.Parallel(t)
	MountCompatible(t)

	// Create root, alloc, and other dirs
	rootDir := t.TempDir()

	clientAllocDir := filepath.Join(rootDir, "nomad")
	require.NoError(t, os.Mkdir(clientAllocDir, fs.ModeDir|0o777))

	otherDir := filepath.Join(rootDir, "etc")
	require.NoError(t, os.Mkdir(otherDir, fs.ModeDir|0o777))

	// chroot contains client.alloc_dir! This could cause infinite
	// recursion.
	chroot := map[string]string{
		rootDir: "/",
	}

	allocDir := NewAllocDir(testlog.HCLogger(t), clientAllocDir, "test")
	taskDir := allocDir.NewTaskDir("testtask")

	require.NoError(t, allocDir.Build())
	defer allocDir.Destroy()

	// Build chroot
	err := taskDir.Build(true, chroot)
	require.NoError(t, err)

	// Assert other directory *was* embedded
	embeddedOtherDir := filepath.Join(clientAllocDir, "test", "testtask", "etc")
	if _, err := os.Stat(embeddedOtherDir); os.IsNotExist(err) {
		t.Fatalf("expected other directory to exist at: %q", embeddedOtherDir)
	}

	// Assert client.alloc_dir was *not* embedded
	embeddedChroot := filepath.Join(clientAllocDir, "test", "testtask", "nomad")
	s, err := os.Stat(embeddedChroot)
	if s != nil {
		t.Logf("somehow you managed to embed the chroot without causing infinite recursion!")
		t.Fatalf("expected chroot to not exist at: %q", embeddedChroot)
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected chroot to not exist but error is: %v", err)
	}
}
