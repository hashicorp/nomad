package executor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	tu "github.com/hashicorp/nomad/testutil"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/stats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	ps "github.com/mitchellh/go-ps"
	"github.com/stretchr/testify/require"
)

var executorFactories = map[string]func(hclog.Logger) Executor{}
var universalFactory = func(l hclog.Logger) Executor {
	return &UniversalExecutor{
		logger:         l,
		processExited:  make(chan interface{}),
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
		pids:           make(map[int]*nomadPid),
	}
}

func init() {
	executorFactories["UniversalExecutor"] = universalFactory
}

// testExecutorContext returns an ExecutorContext and AllocDir.
//
// The caller is responsible for calling AllocDir.Destroy() to cleanup.
func testExecutorCommand(t *testing.T) (*ExecCommand, *allocdir.AllocDir) {
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	taskEnv := env.NewBuilder(mock.Node(), alloc, task, "global").Build()

	allocDir := allocdir.NewAllocDir(testlog.Logger(t), filepath.Join(os.TempDir(), alloc.ID))
	if err := allocDir.Build(); err != nil {
		t.Fatalf("AllocDir.Build() failed: %v", err)
	}
	if err := allocDir.NewTaskDir(task.Name).Build(false, nil, cstructs.FSIsolationNone); err != nil {
		allocDir.Destroy()
		t.Fatalf("allocDir.NewTaskDir(%q) failed: %v", task.Name, err)
	}
	td := allocDir.TaskDirs[task.Name]
	cmd := &ExecCommand{
		Env:     taskEnv.List(),
		TaskDir: td.Dir,
		Resources: &Resources{
			CPU:      task.Resources.CPU,
			MemoryMB: task.Resources.MemoryMB,
			IOPS:     task.Resources.IOPS,
			DiskMB:   task.Resources.DiskMB,
		},
	}
	tl, err := NewTaskLogger(task.Name, &LogConfig{
		LogDir:        td.LogDir,
		MaxFiles:      task.LogConfig.MaxFiles,
		MaxFileSizeMB: task.LogConfig.MaxFileSizeMB,
	}, testLogger(t))
	if err != nil {
		t.Fatalf("NewTaskLogger() failed: %v", err)
	}
	cmd.StdoutFD = tl.StdoutFD()
	cmd.StderrFD = tl.StderrFD()
	return cmd, allocDir
}

func testLogger(t *testing.T) hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Output: testlog.NewWriter(t),
		Level:  hclog.Trace,
	})
}

func TestExecutor_Start_Invalid(pt *testing.T) {
	pt.Parallel()
	invalid := "/bin/foobar"
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			t.Parallel()
			require := require.New(t)
			execCmd, allocDir := testExecutorCommand(t)
			execCmd.Cmd = invalid
			execCmd.Args = []string{"1"}
			defer allocDir.Destroy()
			executor := factory(testLogger(t))
			defer executor.Destroy()

			_, err := executor.Launch(execCmd)
			require.Error(err)
		})
	}
}

func TestExecutor_Start_Wait_Failure_Code(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			t.Parallel()
			require := require.New(t)
			execCmd, allocDir := testExecutorCommand(t)
			execCmd.Cmd = "/bin/date"
			execCmd.Args = []string{"fail"}
			defer allocDir.Destroy()
			executor := factory(testLogger(t))
			defer executor.Destroy()

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)
			ps, _ = executor.Wait()
			require.NotZero(ps.ExitCode, "expected exit code to be non zero")
			require.NoError(executor.Destroy())
		})
	}
}

func TestExecutor_Start_Wait(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			t.Parallel()
			require := require.New(t)
			execCmd, allocDir := testExecutorCommand(t)
			execCmd.Cmd = "/bin/echo"
			execCmd.Args = []string{"hello world"}
			defer allocDir.Destroy()
			executor := factory(testLogger(t))
			defer executor.Destroy()

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)

			ps, err = executor.Wait()
			require.NoError(err)
			require.NoError(executor.Destroy())

			expected := "hello world"
			file := filepath.Join(allocDir.TaskDirs["web"].LogDir, "web.stdout.0")
			tu.WaitForResult(func() (bool, error) {
				output, err := ioutil.ReadFile(file)
				if err != nil {
					return false, err
				}

				act := strings.TrimSpace(string(output))
				if expected != act {
					return false, fmt.Errorf("expected: '%s' actual: '%s'", expected, act)
				}
				return true, nil
			}, func(err error) {
				require.NoError(err)
			})
		})
	}
}

func TestExecutor_WaitExitSignal(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			t.Parallel()
			require := require.New(t)
			execCmd, allocDir := testExecutorCommand(t)
			execCmd.Cmd = "/bin/sleep"
			execCmd.Args = []string{"10000"}
			defer allocDir.Destroy()
			executor := factory(testLogger(t))
			defer executor.Destroy()

			ps, err := executor.Launch(execCmd)
			require.NoError(err)

			go func() {
				time.Sleep(2 * time.Second)
				_, err := executor.Stats()
				require.NoError(err)
				//require.NotEmpty(ru.Pids)
				proc, err := os.FindProcess(ps.Pid)
				require.NoError(err)
				err = proc.Signal(syscall.SIGKILL)
				require.NoError(err)
			}()

			ps, err = executor.Wait()
			require.NoError(err)
			require.Equal(ps.Signal, int(syscall.SIGKILL))
		})
	}
}

func TestExecutor_Start_Kill(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			t.Parallel()
			require := require.New(t)
			execCmd, allocDir := testExecutorCommand(t)
			execCmd.Cmd = "/bin/sleep"
			execCmd.Args = []string{"10 && hello world"}
			defer allocDir.Destroy()
			executor := factory(testLogger(t))
			defer executor.Destroy()

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)

			ps, err = executor.Wait()
			require.NoError(err)
			require.NoError(executor.Destroy())

			file := filepath.Join(allocDir.TaskDirs["web"].LogDir, "web.stdout.0")
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
		})
	}
}

func TestUniversalExecutor_MakeExecutable(t *testing.T) {
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

	// Make a fake executor
	executor := newUniversalExecutor(testLogger(t))

	err = executor.makeExecutable(f.Name())
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

func TestUniversalExecutor_ScanPids(t *testing.T) {
	t.Parallel()
	p1 := NewFakeProcess(2, 5)
	p2 := NewFakeProcess(10, 2)
	p3 := NewFakeProcess(15, 6)
	p4 := NewFakeProcess(3, 10)
	p5 := NewFakeProcess(20, 18)

	// Make a fake executor
	executor := newUniversalExecutor(testLogger(t))

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
