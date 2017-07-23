package executor

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/go-ps"
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

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

// testExecutorContext returns an ExecutorContext and AllocDir.
//
// The caller is responsible for calling AllocDir.Destroy() to cleanup.
func testExecutorContext(t *testing.T) (*ExecutorContext, *allocdir.AllocDir) {
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	taskEnv := env.NewBuilder(mock.Node(), alloc, task, "global").Build()

	allocDir := allocdir.NewAllocDir(testLogger(), filepath.Join(os.TempDir(), alloc.ID))
	if err := allocDir.Build(); err != nil {
		log.Fatalf("AllocDir.Build() failed: %v", err)
	}
	if err := allocDir.NewTaskDir(task.Name).Build(false, nil, cstructs.FSIsolationNone); err != nil {
		allocDir.Destroy()
		log.Fatalf("allocDir.NewTaskDir(%q) failed: %v", task.Name, err)
	}
	td := allocDir.TaskDirs[task.Name]
	ctx := &ExecutorContext{
		TaskEnv: taskEnv,
		Task:    task,
		TaskDir: td.Dir,
		LogDir:  td.LogDir,
	}
	return ctx, allocDir
}

func TestExecutor_Start_Invalid(t *testing.T) {
	t.Parallel()
	invalid := "/bin/foobar"
	execCmd := ExecCommand{Cmd: invalid, Args: []string{"1"}}
	ctx, allocDir := testExecutorContext(t)
	defer allocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))

	if err := executor.SetContext(ctx); err != nil {
		t.Fatalf("Unexpected error")
	}

	if _, err := executor.LaunchCmd(&execCmd); err == nil {
		t.Fatalf("Expected error")
	}
}

func TestExecutor_Start_Wait_Failure_Code(t *testing.T) {
	t.Parallel()
	execCmd := ExecCommand{Cmd: "/bin/date", Args: []string{"fail"}}
	ctx, allocDir := testExecutorContext(t)
	defer allocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))

	if err := executor.SetContext(ctx); err != nil {
		t.Fatalf("Unexpected error")
	}

	ps, err := executor.LaunchCmd(&execCmd)
	if err != nil {
		t.Fatalf("Unexpected error")
	}

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
	t.Parallel()
	execCmd := ExecCommand{Cmd: "/bin/echo", Args: []string{"hello world"}}
	ctx, allocDir := testExecutorContext(t)
	defer allocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))

	if err := executor.SetContext(ctx); err != nil {
		t.Fatalf("Unexpected error")
	}

	ps, err := executor.LaunchCmd(&execCmd)
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
	file := filepath.Join(ctx.LogDir, "web.stdout.0")
	output, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("Couldn't read file %v", file)
	}

	act := strings.TrimSpace(string(output))
	if act != expected {
		t.Fatalf("Command output incorrectly: want %v; got %v", expected, act)
	}
}

func TestExecutor_WaitExitSignal(t *testing.T) {
	t.Parallel()
	execCmd := ExecCommand{Cmd: "/bin/sleep", Args: []string{"10000"}}
	ctx, allocDir := testExecutorContext(t)
	defer allocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))

	if err := executor.SetContext(ctx); err != nil {
		t.Fatalf("Unexpected error")
	}

	ps, err := executor.LaunchCmd(&execCmd)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	go func() {
		time.Sleep(2 * time.Second)
		ru, err := executor.Stats()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(ru.Pids) == 0 {
			t.Fatalf("expected pids")
		}
		proc, err := os.FindProcess(ps.Pid)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if err := proc.Signal(syscall.SIGKILL); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	ps, err = executor.Wait()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ps.Signal != int(syscall.SIGKILL) {
		t.Fatalf("expected signal: %v, actual: %v", int(syscall.SIGKILL), ps.Signal)
	}
}

func TestExecutor_Start_Kill(t *testing.T) {
	t.Parallel()
	execCmd := ExecCommand{Cmd: "/bin/sleep", Args: []string{"10 && hello world"}}
	ctx, allocDir := testExecutorContext(t)
	defer allocDir.Destroy()
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))

	if err := executor.SetContext(ctx); err != nil {
		t.Fatalf("Unexpected error")
	}

	ps, err := executor.LaunchCmd(&execCmd)
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

	file := filepath.Join(ctx.LogDir, "web.stdout.0")
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
	t.Parallel()
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
		t.Fatalf("expected permissions %v; got %v", exp, act)
	}
}

func TestScanPids(t *testing.T) {
	t.Parallel()
	p1 := NewFakeProcess(2, 5)
	p2 := NewFakeProcess(10, 2)
	p3 := NewFakeProcess(15, 6)
	p4 := NewFakeProcess(3, 10)
	p5 := NewFakeProcess(20, 18)

	// Make a fake exececutor
	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags)).(*UniversalExecutor)

	nomadPids, err := executor.scanPids(5, []ps.Process{p1, p2, p3, p4, p5})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(nomadPids) != 4 {
		t.Fatalf("expected: 4, actual: %v", len(nomadPids))
	}
}

type FakeProcess struct {
	pid  int
	ppid int
}

func (f FakeProcess) Pid() int {
	return f.pid
}

func (f FakeProcess) PPid() int {
	return f.ppid
}

func (f FakeProcess) Executable() string {
	return "fake"
}

func NewFakeProcess(pid int, ppid int) ps.Process {
	return FakeProcess{pid: pid, ppid: ppid}
}
