package plugins

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
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
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
	}
	return ctx
}

func TestExecutor_Start_Invalid(t *testing.T) {
	invalid := "/bin/foobar"
	execCmd := ExecCommand{Cmd: invalid, Args: []string{"1"}}
	ctx := testExecutorContext(t)
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))
	_, err := executor.LaunchCmd(&execCmd, ctx)
	if err == nil {
		t.Fatalf("Expected error")
	}
	defer ctx.AllocDir.Destroy()
}

func TestExecutor_Start_Wait_Failure_Code(t *testing.T) {
	execCmd := ExecCommand{Cmd: "/bin/sleep", Args: []string{"fail"}}
	ctx := testExecutorContext(t)
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))
	ps, _ := executor.LaunchCmd(&execCmd, ctx)
	if ps.Pid == 0 {
		t.Fatalf("expected process to start and have non zero pid")
	}
	ps, _ = executor.Wait()
	if ps.ExitCode < 1 {
		t.Fatalf("expected exit code to be non zero, actual: %v", ps.ExitCode)
	}
	defer ctx.AllocDir.Destroy()
}

func TestExecutor_Start_Wait(t *testing.T) {
	execCmd := ExecCommand{Cmd: "/bin/echo", Args: []string{"hello world"}}
	ctx := testExecutorContext(t)
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
	defer ctx.AllocDir.Destroy()

	task := "web"
	taskDir, ok := ctx.AllocDir.TaskDirs[task]
	if !ok {
		log.Panicf("No task directory found for task %v", task)
	}

	expected := "hello world"
	file := filepath.Join(allocdir.TaskLocal, "web.stdout")
	absFilePath := filepath.Join(taskDir, file)
	output, err := ioutil.ReadFile(absFilePath)
	if err != nil {
		t.Fatalf("Couldn't read file %v", absFilePath)
	}

	act := strings.TrimSpace(string(output))
	if act != expected {
		t.Fatalf("Command output incorrectly: want %v; got %v", expected, act)
	}
}

func TestExecutor_Start_Kill(t *testing.T) {
	execCmd := ExecCommand{Cmd: "/bin/sleep", Args: []string{"10 && hello world"}}
	ctx := testExecutorContext(t)
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
	defer ctx.AllocDir.Destroy()

	task := "web"
	taskDir, ok := ctx.AllocDir.TaskDirs[task]
	if !ok {
		t.Fatalf("No task directory found for task %v", task)
	}

	file := filepath.Join(allocdir.TaskLocal, "web.stdout")
	absFilePath := filepath.Join(taskDir, file)

	time.Sleep(time.Duration(testutil.TestMultiplier()*2) * time.Second)

	output, err := ioutil.ReadFile(absFilePath)
	if err != nil {
		t.Fatalf("Couldn't read file %v", absFilePath)
	}

	expected := ""
	act := strings.TrimSpace(string(output))
	if act != expected {
		t.Fatalf("Command output incorrectly: want %v; got %v", expected, act)
	}
}
