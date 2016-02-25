package executor

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
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

func mockAllocDir(t *testing.T) (string, *allocdir.AllocDir) {
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	allocDir := allocdir.NewAllocDir(filepath.Join(os.TempDir(), alloc.ID))
	if err := allocDir.Build([]*structs.Task{task}); err != nil {
		log.Panicf("allocDir.Build() failed: %v", err)
	}

	return task.Name, allocDir
}

func testExecutorContext(t *testing.T) *ExecutorContext {
	taskEnv := env.NewTaskEnvironment(mock.Node())
	taskName, allocDir := mockAllocDir(t)
	ctx := &ExecutorContext{
		TaskEnv:       taskEnv,
		TaskName:      taskName,
		AllocDir:      allocDir,
		TaskResources: constraint,
		LogConfig:     structs.DefaultLogConfig(),
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

	ctx.FSIsolation = true
	ctx.ResourceLimits = true
	ctx.UnprivilegedUser = true

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
