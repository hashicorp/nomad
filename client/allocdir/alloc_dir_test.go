package allocdir

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	t1 = &structs.Task{
		Name:   "web",
		Driver: "exec",
		Config: map[string]string{
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
		Config: map[string]string{
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

	d := NewAllocDir(tmp)
	tasks := []*structs.Task{t1, t2}
	if err := d.Build(tasks); err != nil {
		t.Fatalf("Build(%v) failed: %v", tasks, err)
	}

	// Check that the AllocDir and each of the task directories exist.
	if _, err := os.Stat(d.AllocDir); os.IsNotExist(err) {
		t.Fatalf("Build(%v) didn't create AllocDir %v", tasks, d.AllocDir)
	}

	// Create a file in the alloc dir and then check it exists in each of the
	// task dirs.
	allocFile := "foo"
	allocFileData := []byte{'b', 'a', 'r'}
	if err := ioutil.WriteFile(filepath.Join(d.AllocDir, allocFile), allocFileData, 0777); err != nil {
		t.Fatalf("Couldn't create file in alloc dir: %v", err)
	}

	for _, task := range tasks {
		tDir, err := d.TaskDir(task.Name)
		if err != nil {
			t.Fatalf("TaskDir(%v) failed: %v", task.Name, err)
		}

		if _, err := os.Stat(tDir); os.IsNotExist(err) {
			t.Fatalf("Build(%v) didn't create TaskDir %v", tasks, tDir)
		}

		// TODO: Enable once mount is done.
		//allocExpected := filepath.Join(tDir, SharedAllocName, allocFile)
		//if _, err := os.Stat(allocExpected); os.IsNotExist(err) {
		//t.Fatalf("File in shared alloc dir not accessible from task dir %v: %v", tDir, err)
		//}
	}
}

func TestAllocDir_EmbedNonExistent(t *testing.T) {
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(tmp)
	tasks := []*structs.Task{t1, t2}
	if err := d.Build(tasks); err != nil {
		t.Fatalf("Build(%v) failed: %v", tasks, err)
	}

	fakeDir := "/foobarbaz"
	task := tasks[0].Name
	mapping := map[string]string{fakeDir: fakeDir}
	if err := d.Embed(task, mapping); err == nil {
		t.Fatalf("Embed(%v, %v) should have failed. %v does not exist", task, mapping, fakeDir)
	}
}

func TestAllocDir_EmbedDirs(t *testing.T) {
	tmp, err := ioutil.TempDir("", "AllocDir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	d := NewAllocDir(tmp)
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
	if err := os.Mkdir(subDir, 0777); err != nil {
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
	taskDir, err := d.TaskDir(task)
	if err != nil {
		t.Fatalf("TaskDir(%v) failed: %v", task, err)
	}

	exp := []string{filepath.Join(taskDir, taskDest, file), filepath.Join(taskDir, taskDest, subDirName, subFile)}
	for _, e := range exp {
		if _, err := os.Stat(e); os.IsNotExist(err) {
			t.Fatalf("File %v not embeded: %v", e, err)
		}
	}
}
