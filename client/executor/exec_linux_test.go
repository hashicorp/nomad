package executor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
)

var (
	constraint = &structs.Resources{
		CPU:      0.5,
		MemoryMB: 256,
		Networks: []*structs.NetworkResource{
			&structs.NetworkResource{
				MBits:        50,
				DynamicPorts: []string{"http"},
			},
		},
	}
)

func mockAllocDir(t *testing.T) (string, *allocdir.AllocDir) {
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	allocDir := allocdir.NewAllocDir(filepath.Join(os.TempDir(), alloc.ID))
	if err := allocDir.Build([]*structs.Task{task}); err != nil {
		t.Fatalf("allocDir.Build() failed: %v", err)
	}

	return task.Name, allocDir
}

func TestExecutorLinux_Start_Invalid(t *testing.T) {
	ctestutil.ExecCompatible(t)
	invalid := "/bin/foobar"
	e := Command(invalid, "1")

	if err := e.Limit(constraint); err != nil {
		t.Fatalf("Limit() failed: %v", err)
	}

	task, alloc := mockAllocDir(t)
	defer alloc.Destroy()
	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		t.Fatalf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err == nil {
		t.Fatalf("Start(%v) should have failed", invalid)
	}
}

func TestExecutorLinux_Start_Wait_Failure_Code(t *testing.T) {
	ctestutil.ExecCompatible(t)
	e := Command("/bin/date", "-invalid")

	if err := e.Limit(constraint); err != nil {
		t.Fatalf("Limit() failed: %v", err)
	}

	task, alloc := mockAllocDir(t)
	defer alloc.Destroy()
	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		t.Fatalf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := e.Wait(); err == nil {
		t.Fatalf("Wait() should have failed")
	}
}

func TestExecutorLinux_Start_Wait(t *testing.T) {
	ctestutil.ExecCompatible(t)
	task, alloc := mockAllocDir(t)
	defer alloc.Destroy()

	taskDir, ok := alloc.TaskDirs[task]
	if !ok {
		t.Fatalf("No task directory found for task %v", task)
	}

	expected := "hello world"
	file := filepath.Join(allocdir.TaskLocal, "output.txt")
	absFilePath := filepath.Join(taskDir, file)
	cmd := fmt.Sprintf("%v \"%v\" >> %v", "sleep 1 ; echo -n", expected, file)
	e := Command("/bin/bash", "-c", cmd)

	if err := e.Limit(constraint); err != nil {
		t.Fatalf("Limit() failed: %v", err)
	}

	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		t.Fatalf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := e.Wait(); err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	output, err := ioutil.ReadFile(absFilePath)
	if err != nil {
		t.Fatalf("Couldn't read file %v", absFilePath)
	}

	act := string(output)
	if act != expected {
		t.Fatalf("Command output incorrectly: want %v; got %v", expected, act)
	}
}

func TestExecutorLinux_Start_Kill(t *testing.T) {
	ctestutil.ExecCompatible(t)
	task, alloc := mockAllocDir(t)
	defer alloc.Destroy()

	taskDir, ok := alloc.TaskDirs[task]
	if !ok {
		t.Fatalf("No task directory found for task %v", task)
	}

	filePath := filepath.Join(taskDir, "output")
	e := Command("/bin/bash", "-c", "sleep 1 ; echo \"failure\" > "+filePath)

	// This test can only be run if cgroups are enabled.
	if !e.(*LinuxExecutor).cgroupEnabled {
		t.SkipNow()
	}

	if err := e.Limit(constraint); err != nil {
		t.Fatalf("Limit() failed: %v", err)
	}

	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		t.Fatalf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := e.Shutdown(); err != nil {
		t.Fatalf("Shutdown() failed: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)

	// Check that the file doesn't exist.
	if _, err := os.Stat(filePath); err == nil {
		t.Fatalf("Stat(%v) should have failed: task not killed", filePath)
	}
}

func TestExecutorLinux_Open(t *testing.T) {
	ctestutil.ExecCompatible(t)
	task, alloc := mockAllocDir(t)
	defer alloc.Destroy()

	taskDir, ok := alloc.TaskDirs[task]
	if !ok {
		t.Fatalf("No task directory found for task %v", task)
	}

	filePath := filepath.Join(taskDir, "output")
	e := Command("/bin/bash", "-c", "sleep 1 ; echo \"failure\" > "+filePath)

	// This test can only be run if cgroups are enabled.
	if !e.(*LinuxExecutor).cgroupEnabled {
		t.SkipNow()
	}

	if err := e.Limit(constraint); err != nil {
		t.Fatalf("Limit() failed: %v", err)
	}

	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		t.Fatalf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	id, err := e.ID()
	if err != nil {
		t.Fatalf("ID() failed: %v", err)
	}

	if _, err := OpenId(id); err == nil {
		t.Fatalf("Open(%v) should have failed", id)
	}

	time.Sleep(1500 * time.Millisecond)

	// Check that the file doesn't exist, open should have killed the process.
	if _, err := os.Stat(filePath); err == nil {
		t.Fatalf("Stat(%v) should have failed: task not killed", filePath)
	}
}
