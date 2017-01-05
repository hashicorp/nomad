package allocdir

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// Test that building a chroot will skip nonexistent directories.
func TestTaskDir_EmbedNonExistent(t *testing.T) {
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(testLogger(), tmp)
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
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(testLogger(), tmp)
	defer d.Destroy()
	td := d.NewTaskDir(t1.Name)
	if err := d.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Create a fake host directory, with a file, and a subfolder that contains
	// a file.
	host, err := ioutil.TempDir("", "AllocDirHost")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(host)

	subDirName := "subdir"
	subDir := filepath.Join(host, subDirName)
	if err := os.MkdirAll(subDir, 0777); err != nil {
		t.Fatalf("Failed to make subdir %v: %v", subDir, err)
	}

	file := "foo"
	subFile := "bar"
	if err := ioutil.WriteFile(filepath.Join(host, file), []byte{'a'}, 0777); err != nil {
		t.Fatalf("Coudn't create file in host dir %v: %v", host, err)
	}

	if err := ioutil.WriteFile(filepath.Join(subDir, subFile), []byte{'a'}, 0777); err != nil {
		t.Fatalf("Coudn't create file in host subdir %v: %v", subDir, err)
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
			t.Fatalf("File %v not embeded: %v", f, err)
		}
	}
}
