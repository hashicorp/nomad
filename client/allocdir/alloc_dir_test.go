package allocdir

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tomb "gopkg.in/tomb.v1"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
)

var (
	osMountSharedDirSupport = map[string]bool{
		"darwin": true,
		"linux":  true,
	}

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

func testLogger() *log.Logger {
	if testing.Verbose() {
		return log.New(os.Stderr, "", log.LstdFlags)
	}
	return log.New(ioutil.Discard, "", log.LstdFlags)
}

// Test that AllocDir.Build builds just the alloc directory.
func TestAllocDir_BuildAlloc(t *testing.T) {
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(testLogger(), tmp)
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

func TestAllocDir_MountSharedAlloc(t *testing.T) {
	testutil.MountCompatible(t)
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(testLogger(), tmp)
	defer d.Destroy()
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Build 2 task dirs
	td1 := d.NewTaskDir(t1.Name)
	if err := td1.Build(false, nil, cstructs.FSIsolationChroot); err != nil {
		t.Fatalf("error build task=%q dir: %v", t1.Name, err)
	}
	td2 := d.NewTaskDir(t2.Name)
	if err := td2.Build(false, nil, cstructs.FSIsolationChroot); err != nil {
		t.Fatalf("error build task=%q dir: %v", t2.Name, err)
	}

	// Write a file to the shared dir.
	contents := []byte("foo")
	const filename = "bar"
	if err := ioutil.WriteFile(filepath.Join(d.SharedDir, filename), contents, 0666); err != nil {
		t.Fatalf("Couldn't write file to shared directory: %v", err)
	}

	// Check that the file exists in the task directories
	for _, td := range []*TaskDir{td1, td2} {
		taskFile := filepath.Join(td.SharedTaskDir, filename)
		act, err := ioutil.ReadFile(taskFile)
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
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(testLogger(), tmp)
	defer d.Destroy()
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Build 2 task dirs
	td1 := d.NewTaskDir(t1.Name)
	if err := td1.Build(false, nil, cstructs.FSIsolationImage); err != nil {
		t.Fatalf("error build task=%q dir: %v", t1.Name, err)
	}
	td2 := d.NewTaskDir(t2.Name)
	if err := td2.Build(false, nil, cstructs.FSIsolationImage); err != nil {
		t.Fatalf("error build task=%q dir: %v", t2.Name, err)
	}

	// Write a file to the shared dir.
	exp := []byte{'f', 'o', 'o'}
	file := "bar"
	if err := ioutil.WriteFile(filepath.Join(d.SharedDir, "data", file), exp, 0666); err != nil {
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
	if err := ioutil.WriteFile(filepath.Join(td1.LocalDir, file1), exp, 0666); err != nil {
		t.Fatalf("couldn't write file to task local directory: %v", err)
	}

	// Write a symlink to the task local
	link1 := "baz"
	if err := os.Symlink("bar", filepath.Join(td1.LocalDir, link1)); err != nil {
		t.Fatalf("couldn't write symlink to task local dirctory :%v", err)
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
	tmp1, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp1)

	tmp2, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp2)

	// Create two alloc dirs
	d1 := NewAllocDir(testLogger(), tmp1)
	if err := d1.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	defer d1.Destroy()

	d2 := NewAllocDir(testLogger(), tmp2)
	if err := d2.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	defer d2.Destroy()

	td1 := d1.NewTaskDir(t1.Name)
	if err := td1.Build(false, nil, cstructs.FSIsolationImage); err != nil {
		t.Fatalf("TaskDir.Build() faild: %v", err)
	}

	// Create but don't build second task dir to mimic alloc/task runner
	// behavior (AllocDir.Move() is called pre-TaskDir.Build).
	d2.NewTaskDir(t1.Name)

	dataDir := filepath.Join(d1.SharedDir, SharedDataDir)

	// Write a file to the shared dir.
	exp1 := []byte("foo")
	file1 := "bar"
	if err := ioutil.WriteFile(filepath.Join(dataDir, file1), exp1, 0666); err != nil {
		t.Fatalf("Couldn't write file to shared directory: %v", err)
	}

	// Write a file to the task local
	exp2 := []byte("bar")
	file2 := "lol"
	if err := ioutil.WriteFile(filepath.Join(td1.LocalDir, file2), exp2, 0666); err != nil {
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
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(testLogger(), tmp)
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
	tomb := tomb.Tomb{}
	if _, err := d.BlockUntilExists("../foo", &tomb); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("BlockUntilExists of escaping path didn't error: %v", err)
	}

	// ChangeEvents
	if _, err := d.ChangeEvents("../foo", 0, &tomb); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("ChangeEvents of escaping path didn't error: %v", err)
	}
}

// Test that `nomad fs` can't read secrets
func TestAllocDir_ReadAt_SecretDir(t *testing.T) {
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(testLogger(), tmp)
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	defer d.Destroy()

	td := d.NewTaskDir(t1.Name)
	if err := td.Build(false, nil, cstructs.FSIsolationImage); err != nil {
		t.Fatalf("TaskDir.Build() failed: %v", err)
	}

	// ReadAt of secret dir should fail
	secret := filepath.Join(t1.Name, TaskSecrets, "test_file")
	if _, err := d.ReadAt(secret, 0); err == nil || !strings.Contains(err.Error(), "secret file prohibited") {
		t.Fatalf("ReadAt of secret file didn't error: %v", err)
	}
}

func TestAllocDir_SplitPath(t *testing.T) {
	dir, err := ioutil.TempDir("", "tmpdirtest")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

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
	dir, err := ioutil.TempDir("", "tmpdirtest")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir)

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
	dir1, err := ioutil.TempDir("/tmp", "tempdirdest")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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

// TestAllocDir_Copy asserts that AllocDir.Copy does a deep copy of itself and
// all TaskDirs.
func TestAllocDir_Copy(t *testing.T) {
	a := NewAllocDir(testLogger(), "foo")
	a.NewTaskDir("bar")
	a.NewTaskDir("baz")

	b := a.Copy()
	if diff := pretty.Diff(a, b); len(diff) > 0 {
		t.Errorf("differences between copies: %# v", pretty.Formatter(diff))
	}

	// Make sure TaskDirs map is copied
	a.NewTaskDir("new")
	if b.TaskDirs["new"] != nil {
		t.Errorf("TaskDirs map shared between copied")
	}
}

func TestPathFuncs(t *testing.T) {
	dir, err := ioutil.TempDir("", "nomadtest-pathfuncs")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

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
