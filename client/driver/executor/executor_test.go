package executor

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	tu "github.com/hashicorp/nomad/testutil"
)

var (
	constraint = &structs.Resources{
		CPU:      250,
		MemoryMB: 256,
		Networks: []*structs.NetworkResource{
			&structs.NetworkResource{
				MBits:        50,
				DynamicPorts: []structs.Port{{Label: "http"}},
			},
		},
	}
)

func mockAllocDir(t *testing.T) (*structs.Task, *allocdir.AllocDir) {
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	allocDir := allocdir.NewAllocDir(filepath.Join(os.TempDir(), alloc.ID))
	if err := allocDir.Build([]*structs.Task{task}); err != nil {
		log.Panicf("allocDir.Build() failed: %v", err)
	}

	return task, allocDir
}

func testExecutorContext(t *testing.T) *ExecutorContext {
	taskEnv := env.NewTaskEnvironment(mock.Node())
	task, allocDir := mockAllocDir(t)
	ctx := &ExecutorContext{
		TaskEnv:  taskEnv,
		Task:     task,
		AllocDir: allocDir,
	}
	return ctx
}

func TestExecutor_Start_Invalid(t *testing.T) {
	invalid := "/bin/foobar"
	execCmd := ExecCommand{Cmd: invalid, Args: []string{"1"}}
	ctx := testExecutorContext(t)
	defer ctx.AllocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))
	_, err := executor.LaunchCmd(&execCmd, ctx)
	if err == nil {
		t.Fatalf("Expected error")
	}
}

func TestExecutor_Start_Wait_Failure_Code(t *testing.T) {
	execCmd := ExecCommand{Cmd: "/bin/sleep", Args: []string{"fail"}}
	ctx := testExecutorContext(t)
	defer ctx.AllocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))
	ps, _ := executor.LaunchCmd(&execCmd, ctx)
	if ps.Pid == 0 {
		t.Fatalf("expected process to start and have non zero pid")
	}
	ps, _ = executor.Wait()
	if ps.ExitCode < 1 {
		t.Fatalf("expected exit code to be non zero, actual: %v", ps.ExitCode)
	}
	if err := executor.Exit(); err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestExecutor_Start_Wait(t *testing.T) {
	execCmd := ExecCommand{Cmd: "/bin/echo", Args: []string{"hello world"}}
	ctx := testExecutorContext(t)
	defer ctx.AllocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))
	ps, err := executor.LaunchCmd(&execCmd, ctx)
	if err != nil {
		t.Fatalf("error in launching command: %v", err)
	}
	if ps.Pid == 0 {
		t.Fatalf("expected process to start and have non zero pid")
	}
	ps, err = executor.Wait()
	if err != nil {
		t.Fatalf("error in waiting for command: %v", err)
	}
	if err := executor.Exit(); err != nil {
		t.Fatalf("error: %v", err)
	}

	expected := "hello world"
	file := filepath.Join(ctx.AllocDir.LogDir(), "web.stdout.0")
	output, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("Couldn't read file %v", file)
	}

	act := strings.TrimSpace(string(output))
	if act != expected {
		t.Fatalf("Command output incorrectly: want %v; got %v", expected, act)
	}
}

func TestExecutor_IsolationAndConstraints(t *testing.T) {
	testutil.ExecCompatible(t)

	execCmd := ExecCommand{Cmd: "/bin/echo", Args: []string{"hello world"}}
	ctx := testExecutorContext(t)
	defer ctx.AllocDir.Destroy()

	execCmd.FSIsolation = true
	execCmd.ResourceLimits = true
	execCmd.User = cstructs.DefaultUnpriviledgedUser

	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))
	ps, err := executor.LaunchCmd(&execCmd, ctx)
	if err != nil {
		t.Fatalf("error in launching command: %v", err)
	}
	if ps.Pid == 0 {
		t.Fatalf("expected process to start and have non zero pid")
	}
	ps, err = executor.Wait()
	if err != nil {
		t.Fatalf("error in waiting for command: %v", err)
	}
	if err := executor.Exit(); err != nil {
		t.Fatalf("error: %v", err)
	}

	expected := "hello world"
	file := filepath.Join(ctx.AllocDir.LogDir(), "web.stdout.0")
	output, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("Couldn't read file %v", file)
	}

	act := strings.TrimSpace(string(output))
	if act != expected {
		t.Fatalf("Command output incorrectly: want %v; got %v", expected, act)
	}
}

func TestExecutor_DestroyCgroup(t *testing.T) {
	testutil.ExecCompatible(t)

	execCmd := ExecCommand{Cmd: "/bin/bash", Args: []string{"-c", "/usr/bin/yes"}}
	ctx := testExecutorContext(t)
	ctx.Task.LogConfig.MaxFiles = 1
	ctx.Task.LogConfig.MaxFileSizeMB = 300
	defer ctx.AllocDir.Destroy()

	execCmd.FSIsolation = true
	execCmd.ResourceLimits = true
	execCmd.User = "nobody"

	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))
	ps, err := executor.LaunchCmd(&execCmd, ctx)
	if err != nil {
		t.Fatalf("error in launching command: %v", err)
	}
	if ps.Pid == 0 {
		t.Fatalf("expected process to start and have non zero pid")
	}
	time.Sleep(200 * time.Millisecond)
	if err := executor.Exit(); err != nil {
		t.Fatalf("err: %v", err)
	}

	file := filepath.Join(ctx.AllocDir.LogDir(), "web.stdout.0")
	finfo, err := os.Stat(file)
	if err != nil {
		t.Fatalf("error stating stdout file: %v", err)
	}
	time.Sleep(1 * time.Second)
	finfo1, err := os.Stat(file)
	if err != nil {
		t.Fatalf("error stating stdout file: %v", err)
	}
	if finfo.Size() != finfo1.Size() {
		t.Fatalf("Expected size: %v, actual: %v", finfo.Size(), finfo1.Size())
	}
}

func TestExecutor_Start_Kill(t *testing.T) {
	execCmd := ExecCommand{Cmd: "/bin/sleep", Args: []string{"10 && hello world"}}
	ctx := testExecutorContext(t)
	defer ctx.AllocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))
	ps, err := executor.LaunchCmd(&execCmd, ctx)
	if err != nil {
		t.Fatalf("error in launching command: %v", err)
	}
	if ps.Pid == 0 {
		t.Fatalf("expected process to start and have non zero pid")
	}
	ps, err = executor.Wait()
	if err != nil {
		t.Fatalf("error in waiting for command: %v", err)
	}
	if err := executor.Exit(); err != nil {
		t.Fatalf("error: %v", err)
	}

	file := filepath.Join(ctx.AllocDir.LogDir(), "web.stdout.0")
	time.Sleep(time.Duration(tu.TestMultiplier()*2) * time.Second)

	output, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("Couldn't read file %v", file)
	}

	expected := ""
	act := strings.TrimSpace(string(output))
	if act != expected {
		t.Fatalf("Command output incorrectly: want %v; got %v", expected, act)
	}
}

func TestExecutor_MakeExecutable(t *testing.T) {
	// Create a temp file
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	defer os.Remove(f.Name())

	// Set its permissions to be non-executable
	f.Chmod(os.FileMode(0610))

	// Make a fake exececutor
	ctx := testExecutorContext(t)
	defer ctx.AllocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))

	err = executor.(*UniversalExecutor).makeExecutable(f.Name())
	if err != nil {
		t.Fatalf("makeExecutable() failed: %v", err)
	}

	// Check the permissions
	stat, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat() failed: %v", err)
	}

	act := stat.Mode().Perm()
	exp := os.FileMode(0755)
	if act != exp {
		t.Fatalf("expected permissions %v; got %v", err)
	}
}

func TestExecutorInterpolateServices(t *testing.T) {
	task := mock.Job().TaskGroups[0].Tasks[0]
	// Make a fake exececutor
	ctx := testExecutorContext(t)
	defer ctx.AllocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))

	executor.(*UniversalExecutor).ctx = ctx
	executor.(*UniversalExecutor).interpolateServices(task)
	expected := []string{"pci:true", "datacenter:dc1"}
	if !reflect.DeepEqual(task.Services[0].Tags, expected) {
		t.Fatalf("expected: %v, actual: %v", expected, task.Services[0].Tags)
	}
}
