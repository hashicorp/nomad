package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	tu "github.com/hashicorp/nomad/testutil"
	ps "github.com/mitchellh/go-ps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var executorFactories = map[string]executorFactory{}

type executorFactory struct {
	new              func(hclog.Logger) Executor
	configureExecCmd func(*testing.T, *ExecCommand)
}

var universalFactory = executorFactory{
	new:              NewExecutor,
	configureExecCmd: func(*testing.T, *ExecCommand) {},
}

func init() {
	executorFactories["UniversalExecutor"] = universalFactory
}

type testExecCmd struct {
	command  *ExecCommand
	allocDir *allocdir.AllocDir

	stdout         *bytes.Buffer
	stderr         *bytes.Buffer
	outputCopyDone *sync.WaitGroup
}

// testExecutorContext returns an ExecutorContext and AllocDir.
//
// The caller is responsible for calling AllocDir.Destroy() to cleanup.
func testExecutorCommand(t *testing.T) *testExecCmd {
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	taskEnv := taskenv.NewBuilder(mock.Node(), alloc, task, "global").Build()

	allocDir := allocdir.NewAllocDir(testlog.HCLogger(t), filepath.Join(os.TempDir(), alloc.ID))
	if err := allocDir.Build(); err != nil {
		t.Fatalf("AllocDir.Build() failed: %v", err)
	}
	if err := allocDir.NewTaskDir(task.Name).Build(false, nil); err != nil {
		allocDir.Destroy()
		t.Fatalf("allocDir.NewTaskDir(%q) failed: %v", task.Name, err)
	}
	td := allocDir.TaskDirs[task.Name]
	cmd := &ExecCommand{
		Env:     taskEnv.List(),
		TaskDir: td.Dir,
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 500,
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 256,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				CPUShares:        500,
				MemoryLimitBytes: 256 * 1024 * 1024,
			},
		},
	}

	testCmd := &testExecCmd{
		command:  cmd,
		allocDir: allocDir,
	}
	configureTLogging(t, testCmd)
	return testCmd
}

// configureTLogging configures a test command executor with buffer as Std{out|err}
// but using os.Pipe so it mimics non-test case where cmd is set with files as Std{out|err}
// the buffers can be used to read command output
func configureTLogging(t *testing.T, testcmd *testExecCmd) {
	var stdout, stderr bytes.Buffer
	var copyDone sync.WaitGroup

	stdoutPr, stdoutPw, err := os.Pipe()
	require.NoError(t, err)

	stderrPr, stderrPw, err := os.Pipe()
	require.NoError(t, err)

	copyDone.Add(2)
	go func() {
		defer copyDone.Done()
		io.Copy(&stdout, stdoutPr)
	}()
	go func() {
		defer copyDone.Done()
		io.Copy(&stderr, stderrPr)
	}()

	testcmd.stdout = &stdout
	testcmd.stderr = &stderr
	testcmd.outputCopyDone = &copyDone

	testcmd.command.stdout = stdoutPw
	testcmd.command.stderr = stderrPw
	return
}

func TestExecutor_Start_Invalid(pt *testing.T) {
	pt.Parallel()
	invalid := "/bin/foobar"
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			require := require.New(t)
			testExecCmd := testExecutorCommand(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			execCmd.Cmd = invalid
			execCmd.Args = []string{"1"}
			factory.configureExecCmd(t, execCmd)
			defer allocDir.Destroy()
			executor := factory.new(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			_, err := executor.Launch(execCmd)
			require.Error(err)
		})
	}
}

func TestExecutor_Start_Wait_Failure_Code(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			require := require.New(t)
			testExecCmd := testExecutorCommand(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			execCmd.Cmd = "/bin/sh"
			execCmd.Args = []string{"-c", "sleep 1; /bin/date fail"}
			factory.configureExecCmd(t, execCmd)
			defer allocDir.Destroy()
			executor := factory.new(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)
			ps, _ = executor.Wait(context.Background())
			require.NotZero(ps.ExitCode, "expected exit code to be non zero")
			require.NoError(executor.Shutdown("SIGINT", 100*time.Millisecond))
		})
	}
}

func TestExecutor_Start_Wait(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			require := require.New(t)
			testExecCmd := testExecutorCommand(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			execCmd.Cmd = "/bin/echo"
			execCmd.Args = []string{"hello world"}
			factory.configureExecCmd(t, execCmd)

			defer allocDir.Destroy()
			executor := factory.new(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)

			ps, err = executor.Wait(context.Background())
			require.NoError(err)
			require.NoError(executor.Shutdown("SIGINT", 100*time.Millisecond))

			expected := "hello world"
			tu.WaitForResult(func() (bool, error) {
				act := strings.TrimSpace(string(testExecCmd.stdout.String()))
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

func TestExecutor_Start_Wait_Children(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			require := require.New(t)
			testExecCmd := testExecutorCommand(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			execCmd.Cmd = "/bin/sh"
			execCmd.Args = []string{"-c", "(sleep 30 > /dev/null & ) ; exec sleep 1"}
			factory.configureExecCmd(t, execCmd)

			defer allocDir.Destroy()
			executor := factory.new(testlog.HCLogger(t))
			defer executor.Shutdown("SIGKILL", 0)

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)

			ch := make(chan error)

			go func() {
				ps, err = executor.Wait(context.Background())
				t.Logf("Processe completed with %#v error: %#v", ps, err)
				ch <- err
			}()

			timeout := 7 * time.Second
			select {
			case <-ch:
				require.NoError(err)
				//good
			case <-time.After(timeout):
				require.Fail(fmt.Sprintf("process is running after timeout: %v", timeout))
			}
		})
	}
}

func TestExecutor_WaitExitSignal(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			require := require.New(t)
			testExecCmd := testExecutorCommand(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			execCmd.Cmd = "/bin/sleep"
			execCmd.Args = []string{"10000"}
			execCmd.ResourceLimits = true
			factory.configureExecCmd(t, execCmd)

			defer allocDir.Destroy()
			executor := factory.new(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			ps, err := executor.Launch(execCmd)
			require.NoError(err)

			go func() {
				tu.WaitForResult(func() (bool, error) {
					ch, err := executor.Stats(context.Background(), time.Second)
					if err != nil {
						return false, err
					}
					select {
					case <-time.After(time.Second):
						return false, fmt.Errorf("stats failed to send on interval")
					case ru := <-ch:
						assert.NotEmpty(t, ru.Pids, "no pids recorded in stats")
						assert.NotZero(t, ru.ResourceUsage.MemoryStats.RSS)
						assert.WithinDuration(t, time.Now(), time.Unix(0, ru.Timestamp), time.Second)
					}
					proc, err := os.FindProcess(ps.Pid)
					if err != nil {
						return false, err
					}
					err = proc.Signal(syscall.SIGKILL)
					if err != nil {
						return false, err
					}
					return true, nil
				}, func(err error) {
					assert.NoError(t, executor.Signal(os.Kill))
					assert.NoError(t, err)
				})
			}()

			ps, err = executor.Wait(context.Background())
			require.NoError(err)
			require.Equal(ps.Signal, int(syscall.SIGKILL))
		})
	}
}

func TestExecutor_Start_Kill(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			require := require.New(t)
			testExecCmd := testExecutorCommand(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			execCmd.Cmd = "/bin/sleep"
			execCmd.Args = []string{"10"}
			factory.configureExecCmd(t, execCmd)

			defer allocDir.Destroy()
			executor := factory.new(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)

			require.NoError(executor.Shutdown("SIGINT", 100*time.Millisecond))

			time.Sleep(time.Duration(tu.TestMultiplier()*2) * time.Second)
			output := testExecCmd.stdout.String()
			expected := ""
			act := strings.TrimSpace(string(output))
			if act != expected {
				t.Fatalf("Command output incorrectly: want %v; got %v", expected, act)
			}
		})
	}
}

func TestExecutor_Shutdown_Exit(t *testing.T) {
	require := require.New(t)
	t.Parallel()
	testExecCmd := testExecutorCommand(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "/bin/sleep"
	execCmd.Args = []string{"100"}
	cfg := &ExecutorConfig{
		LogFile: "/dev/null",
	}
	executor, pluginClient, err := CreateExecutor(testlog.HCLogger(t), nil, cfg)
	require.NoError(err)

	proc, err := executor.Launch(execCmd)
	require.NoError(err)
	require.NotZero(proc.Pid)

	executor.Shutdown("", 0)
	pluginClient.Kill()
	tu.WaitForResult(func() (bool, error) {
		p, err := ps.FindProcess(proc.Pid)
		if err != nil {
			return false, err
		}
		return p == nil, fmt.Errorf("process found: %d", proc.Pid)
	}, func(err error) {
		require.NoError(err)
	})
	require.NoError(allocDir.Destroy())
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

	err = makeExecutable(f.Name())
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

func TestUniversalExecutor_LookupPath(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	// Create a temp dir
	tmpDir, err := ioutil.TempDir("", "")
	require.Nil(err)
	defer os.Remove(tmpDir)

	// Make a foo subdir
	os.MkdirAll(filepath.Join(tmpDir, "foo"), 0700)

	// Write a file under foo
	filePath := filepath.Join(tmpDir, "foo", "tmp.txt")
	err = ioutil.WriteFile(filePath, []byte{1, 2}, os.ModeAppend)
	require.Nil(err)

	// Lookup with full path on host to binary
	path, err := lookupBin("not_tmpDir", filePath)
	require.Nil(err)
	require.Equal(filePath, path)

	// Lookout with an absolute path to the binary
	_, err = lookupBin(tmpDir, "/foo/tmp.txt")
	require.Nil(err)

	// Write a file under task dir
	filePath3 := filepath.Join(tmpDir, "tmp.txt")
	ioutil.WriteFile(filePath3, []byte{1, 2}, os.ModeAppend)

	// Lookup with file name, should find the one we wrote above
	path, err = lookupBin(tmpDir, "tmp.txt")
	require.Nil(err)
	require.Equal(filepath.Join(tmpDir, "tmp.txt"), path)

	// Write a file under local subdir
	os.MkdirAll(filepath.Join(tmpDir, "local"), 0700)
	filePath2 := filepath.Join(tmpDir, "local", "tmp.txt")
	ioutil.WriteFile(filePath2, []byte{1, 2}, os.ModeAppend)

	// Lookup with file name, should find the one we wrote above
	path, err = lookupBin(tmpDir, "tmp.txt")
	require.Nil(err)
	require.Equal(filepath.Join(tmpDir, "local", "tmp.txt"), path)

	// Lookup a host path
	_, err = lookupBin(tmpDir, "/bin/sh")
	require.NoError(err)

	// Lookup a host path via $PATH
	_, err = lookupBin(tmpDir, "sh")
	require.NoError(err)
}

// setupRoootfs setups the rootfs for libcontainer executor
// It uses busybox to make some binaries available - somewhat cheaper
// than mounting the underlying host filesystem
func setupRootfs(t *testing.T, rootfs string) {
	paths := []string{
		"/bin/sh",
		"/bin/sleep",
		"/bin/echo",
		"/bin/date",
	}

	for _, p := range paths {
		setupRootfsBinary(t, rootfs, p)
	}
}

// setupRootfsBinary installs a busybox link in the desired path
func setupRootfsBinary(t *testing.T, rootfs, path string) {
	t.Helper()

	dst := filepath.Join(rootfs, path)
	err := os.MkdirAll(filepath.Dir(dst), 0755)
	require.NoError(t, err)

	src := filepath.Join(
		"test-resources", "busybox",
		fmt.Sprintf("busybox-%s", runtime.GOARCH),
	)

	err = os.Link(src, dst)
	require.NoError(t, err)
}

// TestExecutor_Start_Kill_Immediately_NoGrace asserts that executors shutdown
// immediately when sent a kill signal with no grace period.
func TestExecutor_Start_Kill_Immediately_NoGrace(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			require := require.New(t)
			testExecCmd := testExecutorCommand(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			execCmd.Cmd = "/bin/sleep"
			execCmd.Args = []string{"100"}
			factory.configureExecCmd(t, execCmd)
			defer allocDir.Destroy()
			executor := factory.new(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)

			waitCh := make(chan interface{})
			go func() {
				defer close(waitCh)
				executor.Wait(context.Background())
			}()

			require.NoError(executor.Shutdown("SIGKILL", 0))

			select {
			case <-waitCh:
				// all good!
			case <-time.After(4 * time.Second * time.Duration(tu.TestMultiplier())):
				require.Fail("process did not terminate despite SIGKILL")
			}
		})
	}
}

func TestExecutor_Start_Kill_Immediately_WithGrace(pt *testing.T) {
	pt.Parallel()
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			require := require.New(t)
			testExecCmd := testExecutorCommand(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			execCmd.Cmd = "/bin/sleep"
			execCmd.Args = []string{"100"}
			factory.configureExecCmd(t, execCmd)
			defer allocDir.Destroy()
			executor := factory.new(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)

			waitCh := make(chan interface{})
			go func() {
				defer close(waitCh)
				executor.Wait(context.Background())
			}()

			require.NoError(executor.Shutdown("SIGKILL", 100*time.Millisecond))

			select {
			case <-waitCh:
				// all good!
			case <-time.After(4 * time.Second * time.Duration(tu.TestMultiplier())):
				require.Fail("process did not terminate despite SIGKILL")
			}
		})
	}
}

// TestExecutor_Start_NonExecutableBinaries asserts that executor marks binary as executable
// before starting
func TestExecutor_Start_NonExecutableBinaries(pt *testing.T) {
	pt.Parallel()

	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			require := require.New(t)

			tmpDir, err := ioutil.TempDir("", "nomad-executor-tests")
			require.NoError(err)
			defer os.RemoveAll(tmpDir)

			nonExecutablePath := filepath.Join(tmpDir, "nonexecutablefile")
			ioutil.WriteFile(nonExecutablePath,
				[]byte("#!/bin/sh\necho hello world"),
				0600)

			testExecCmd := testExecutorCommand(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			execCmd.Cmd = nonExecutablePath
			factory.configureExecCmd(t, execCmd)

			executor := factory.new(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			// need to configure path in chroot with that file if using isolation executor
			if _, ok := executor.(*UniversalExecutor); !ok {
				taskName := filepath.Base(testExecCmd.command.TaskDir)
				err := allocDir.NewTaskDir(taskName).Build(true, map[string]string{
					tmpDir: tmpDir,
				})
				require.NoError(err)
			}

			defer allocDir.Destroy()
			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)

			ps, err = executor.Wait(context.Background())
			require.NoError(err)
			require.NoError(executor.Shutdown("SIGINT", 100*time.Millisecond))

			expected := "hello world"
			tu.WaitForResult(func() (bool, error) {
				act := strings.TrimSpace(string(testExecCmd.stdout.String()))
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
