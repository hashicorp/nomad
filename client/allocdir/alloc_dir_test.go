package allocdir

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/nomad/structs"
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

// Test that given a set of tasks, each task gets a directory and that directory
// has the shared alloc dir inside of it.
func TestAllocDir_BuildAlloc(t *testing.T) {
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(tmp, structs.DefaultResources().DiskMB)
	defer d.Destroy()
	tasks := []*structs.Task{t1, t2}
	if err := d.Build(tasks); err != nil {
		t.Fatalf("Build(%v) failed: %v", tasks, err)
	}

	// Check that the AllocDir and each of the task directories exist.
	if _, err := os.Stat(d.AllocDir); os.IsNotExist(err) {
		t.Fatalf("Build(%v) didn't create AllocDir %v", tasks, d.AllocDir)
	}

	for _, task := range tasks {
		tDir, ok := d.TaskDirs[task.Name]
		if !ok {
			t.Fatalf("Task directory not found for %v", task.Name)
		}

		if _, err := os.Stat(tDir); os.IsNotExist(err) {
			t.Fatalf("Build(%v) didn't create TaskDir %v", tasks, tDir)
		}

		if _, err := os.Stat(filepath.Join(tDir, TaskSecrets)); os.IsNotExist(err) {
			t.Fatalf("Build(%v) didn't create secret dir %v", tasks)
		}
	}
}

func TestAllocDir_LogDir(t *testing.T) {
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(tmp, structs.DefaultResources().DiskMB)
	defer d.Destroy()

	expected := filepath.Join(d.AllocDir, SharedAllocName, LogDirName)
	if d.LogDir() != expected {
		t.Fatalf("expected: %v, got: %v", expected, d.LogDir())
	}
}

func TestAllocDir_EmbedNonExistent(t *testing.T) {
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(tmp, structs.DefaultResources().DiskMB)
	defer d.Destroy()
	tasks := []*structs.Task{t1, t2}
	if err := d.Build(tasks); err != nil {
		t.Fatalf("Build(%v) failed: %v", tasks, err)
	}

	fakeDir := "/foobarbaz"
	task := tasks[0].Name
	mapping := map[string]string{fakeDir: fakeDir}
	if err := d.Embed(task, mapping); err != nil {
		t.Fatalf("Embed(%v, %v) should should skip %v since it does not exist", task, mapping, fakeDir)
	}
}

func TestAllocDir_EmbedDirs(t *testing.T) {
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(tmp, structs.DefaultResources().DiskMB)
	defer d.Destroy()
	tasks := []*structs.Task{t1, t2}
	if err := d.Build(tasks); err != nil {
		t.Fatalf("Build(%v) failed: %v", tasks, err)
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
	task := tasks[0].Name
	taskDest := "bin/test/"
	mapping := map[string]string{host: taskDest}
	if err := d.Embed(task, mapping); err != nil {
		t.Fatalf("Embed(%v, %v) failed: %v", task, mapping, err)
	}

	// Check that the embedding was done properly.
	taskDir, ok := d.TaskDirs[task]
	if !ok {
		t.Fatalf("Task directory not found for %v", task)
	}

	exp := []string{filepath.Join(taskDir, taskDest, file), filepath.Join(taskDir, taskDest, subDirName, subFile)}
	for _, e := range exp {
		if _, err := os.Stat(e); os.IsNotExist(err) {
			t.Fatalf("File %v not embeded: %v", e, err)
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

	d := NewAllocDir(tmp, structs.DefaultResources().DiskMB)
	defer d.Destroy()
	tasks := []*structs.Task{t1, t2}
	if err := d.Build(tasks); err != nil {
		t.Fatalf("Build(%v) failed: %v", tasks, err)
	}

	// Write a file to the shared dir.
	exp := []byte{'f', 'o', 'o'}
	file := "bar"
	if err := ioutil.WriteFile(filepath.Join(d.SharedDir, file), exp, 0777); err != nil {
		t.Fatalf("Couldn't write file to shared directory: %v", err)
	}

	for _, task := range tasks {
		// Mount and then check that the file exists in the task directory.
		if err := d.MountSharedDir(task.Name); err != nil {
			if v, ok := osMountSharedDirSupport[runtime.GOOS]; v && ok {
				t.Fatalf("MountSharedDir(%v) failed: %v", task.Name, err)
			} else {
				t.Skipf("MountShareDir(%v) failed, no OS support")
			}
		}

		taskDir, ok := d.TaskDirs[task.Name]
		if !ok {
			t.Fatalf("Task directory not found for %v", task.Name)
		}

		taskFile := filepath.Join(taskDir, SharedAllocName, file)
		act, err := ioutil.ReadFile(taskFile)
		if err != nil {
			t.Fatalf("Failed to read shared alloc file from task dir: %v", err)
		}

		if !reflect.DeepEqual(act, exp) {
			t.Fatalf("Incorrect data read from task dir: want %v; got %v", exp, act)
		}
	}
}
