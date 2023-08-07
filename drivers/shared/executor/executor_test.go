// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	tu "github.com/hashicorp/nomad/testutil"
	ps "github.com/mitchellh/go-ps"
	"github.com/shoenig/test/must"
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

	allocDir := allocdir.NewAllocDir(testlog.HCLogger(t), t.TempDir(), alloc.ID)
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
				CpusetCgroupPath: cgroupslib.LinuxResourcesPath(alloc.ID, task.Name),
			},
		},
	}

	// create cgroup for our task (because we aren't using task runners)
	f := cgroupslib.Factory(alloc.ID, task.Name)
	must.NoError(t, f.Setup())

	// cleanup cgroup once test is done (because no task runners)
	t.Cleanup(func() {
		_ = f.Kill()
		_ = f.Teardown()
	})

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

func TestExecutor_Start_Invalid(t *testing.T) {
	ci.Parallel(t)
	invalid := "/bin/foobar"
	for name, factory := range executorFactories {
		t.Run(name, func(t *testing.T) {
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

func TestExecutor_Start_Wait_Failure_Code(t *testing.T) {
	ci.Parallel(t)
	for name, factory := range executorFactories {
		t.Run(name, func(t *testing.T) {
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

func TestExecutor_Start_Wait(t *testing.T) {
	ci.Parallel(t)
	for name, factory := range executorFactories {
		t.Run(name, func(t *testing.T) {
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

func TestExecutor_Start_Wait_Children(t *testing.T) {
	ci.Parallel(t)
	for name, factory := range executorFactories {
		t.Run(name, func(t *testing.T) {
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

func TestExecutor_WaitExitSignal(t *testing.T) {
	ci.Parallel(t)
	testutil.CgroupsCompatibleV1(t) // todo(shoenig) #12351

	for name, factory := range executorFactories {
		t.Run(name, func(t *testing.T) {
			testExecCmd := testExecutorCommand(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			execCmd.Cmd = "/bin/sleep"
			execCmd.Args = []string{"10000"}
			execCmd.ResourceLimits = true
			factory.configureExecCmd(t, execCmd)

			defer allocDir.Destroy()
			executor := factory.new(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			pState, err := executor.Launch(execCmd)
			require.NoError(t, err)

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

						// just checking we measured something; each executor type has its own abilities,
						// and e.g. cgroup v2 provides different information than cgroup v1
						assert.NotEmpty(t, ru.ResourceUsage.MemoryStats.Measured)

						assert.WithinDuration(t, time.Now(), time.Unix(0, ru.Timestamp), time.Second)
					}
					proc, err := os.FindProcess(pState.Pid)
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

			pState, err = executor.Wait(context.Background())
			require.NoError(t, err)
			require.Equal(t, pState.Signal, int(syscall.SIGKILL))
		})
	}
}

func TestExecutor_Start_Kill(t *testing.T) {
	ci.Parallel(t)
	for name, factory := range executorFactories {
		t.Run(name, func(t *testing.T) {
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
	ci.Parallel(t)
	require := require.New(t)
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
	ci.Parallel(t)
	// Create a temp file
	f, err := os.CreateTemp("", "")
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
	ci.Parallel(t)
	require := require.New(t)
	// Create a temp dir
	tmpDir := t.TempDir()

	// Make a foo subdir
	os.MkdirAll(filepath.Join(tmpDir, "foo"), 0700)

	// Write a file under foo
	filePath := filepath.Join(tmpDir, "foo", "tmp.txt")
	err := os.WriteFile(filePath, []byte{1, 2}, os.ModeAppend)
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
	os.WriteFile(filePath3, []byte{1, 2}, os.ModeAppend)

	// Lookup with file name, should find the one we wrote above
	path, err = lookupBin(tmpDir, "tmp.txt")
	require.Nil(err)
	require.Equal(filepath.Join(tmpDir, "tmp.txt"), path)

	// Write a file under local subdir
	os.MkdirAll(filepath.Join(tmpDir, "local"), 0700)
	filePath2 := filepath.Join(tmpDir, "local", "tmp.txt")
	os.WriteFile(filePath2, []byte{1, 2}, os.ModeAppend)

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
	if err != nil {
		// On failure, fallback to copying the file directly.
		// Linking may fail if the test source code lives on a separate
		// volume/partition from the temp dir used for testing
		copyFile(t, src, dst)
	}
}

func copyFile(t *testing.T, src, dst string) {
	in, err := os.Open(src)
	require.NoErrorf(t, err, "copying %v -> %v", src, dst)
	defer in.Close()

	ins, err := in.Stat()
	require.NoErrorf(t, err, "copying %v -> %v", src, dst)

	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, ins.Mode())
	require.NoErrorf(t, err, "copying %v -> %v", src, dst)
	defer func() {
		if err := out.Close(); err != nil {
			t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
		}
	}()

	_, err = io.Copy(out, in)
	require.NoErrorf(t, err, "copying %v -> %v", src, dst)
}

// TestExecutor_Start_Kill_Immediately_NoGrace asserts that executors shutdown
// immediately when sent a kill signal with no grace period.
func TestExecutor_Start_Kill_Immediately_NoGrace(t *testing.T) {
	ci.Parallel(t)
	for name, factory := range executorFactories {

		t.Run(name, func(t *testing.T) {
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

func TestExecutor_Start_Kill_Immediately_WithGrace(t *testing.T) {
	ci.Parallel(t)
	for name, factory := range executorFactories {
		t.Run(name, func(t *testing.T) {
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
func TestExecutor_Start_NonExecutableBinaries(t *testing.T) {
	ci.Parallel(t)

	for name, factory := range executorFactories {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			tmpDir := t.TempDir()

			nonExecutablePath := filepath.Join(tmpDir, "nonexecutablefile")
			os.WriteFile(nonExecutablePath,
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
				stderr := strings.TrimSpace(string(testExecCmd.stderr.String()))
				t.Logf("stderr: %v", stderr)
				require.NoError(err)
			})
		})
	}
}
