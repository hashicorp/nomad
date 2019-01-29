package executor

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/plugins/drivers"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var executorFactories = map[string]func(hclog.Logger) Executor{}
var universalFactory = func(l hclog.Logger) Executor {
	return NewExecutor(l)
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
			NomadResources: alloc.AllocatedResources.Tasks[task.Name],
		},
	}

	setupRootfs(t, td.Dir)
	configureTLogging(cmd)
	return cmd, allocDir
}

type bufferCloser struct {
	bytes.Buffer
}

func (_ *bufferCloser) Close() error { return nil }

func configureTLogging(cmd *ExecCommand) (stdout bufferCloser, stderr bufferCloser) {
	cmd.SetWriters(&stdout, &stderr)
	return
}

func TestExecutor_Start_Invalid(pt *testing.T) {
	pt.Parallel()
	invalid := "/bin/foobar"
	for name, factory := range executorFactories {
		pt.Run(name, func(t *testing.T) {
			require := require.New(t)
			execCmd, allocDir := testExecutorCommand(t)
			execCmd.Cmd = invalid
			execCmd.Args = []string{"1"}
			defer allocDir.Destroy()
			executor := factory(testlog.HCLogger(t))
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
			execCmd, allocDir := testExecutorCommand(t)
			execCmd.Cmd = "/bin/date"
			execCmd.Args = []string{"fail"}
			defer allocDir.Destroy()
			executor := factory(testlog.HCLogger(t))
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
			execCmd, allocDir := testExecutorCommand(t)
			execCmd.Cmd = "/bin/echo"
			execCmd.Args = []string{"hello world"}

			defer allocDir.Destroy()
			executor := factory(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)

			ps, err = executor.Wait(context.Background())
			require.NoError(err)
			require.NoError(executor.Shutdown("SIGINT", 100*time.Millisecond))

			expected := "hello world"
			tu.WaitForResult(func() (bool, error) {
				outWriter, _ := execCmd.GetWriters()
				output := outWriter.(*bufferCloser).String()
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
			require := require.New(t)
			execCmd, allocDir := testExecutorCommand(t)
			execCmd.Cmd = "/bin/sleep"
			execCmd.Args = []string{"10000"}
			execCmd.ResourceLimits = true
			defer allocDir.Destroy()
			executor := factory(testlog.HCLogger(t))
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
			execCmd, allocDir := testExecutorCommand(t)
			execCmd.Cmd = "/bin/sleep"
			execCmd.Args = []string{"10"}
			defer allocDir.Destroy()
			executor := factory(testlog.HCLogger(t))
			defer executor.Shutdown("", 0)

			ps, err := executor.Launch(execCmd)
			require.NoError(err)
			require.NotZero(ps.Pid)

			require.NoError(executor.Shutdown("SIGINT", 100*time.Millisecond))

			time.Sleep(time.Duration(tu.TestMultiplier()*2) * time.Second)
			outWriter, _ := execCmd.GetWriters()
			output := outWriter.(*bufferCloser).String()
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
	// Create a temp dir
	tmpDir, err := ioutil.TempDir("", "")
	require := require.New(t)
	require.Nil(err)
	defer os.Remove(tmpDir)

	// Make a foo subdir
	os.MkdirAll(filepath.Join(tmpDir, "foo"), 0700)

	// Write a file under foo
	filePath := filepath.Join(tmpDir, "foo", "tmp.txt")
	err = ioutil.WriteFile(filePath, []byte{1, 2}, os.ModeAppend)
	require.Nil(err)

	// Lookup with full path to binary
	_, err = lookupBin("dummy", filePath)
	require.Nil(err)

	// Write a file under local subdir
	os.MkdirAll(filepath.Join(tmpDir, "local"), 0700)
	filePath2 := filepath.Join(tmpDir, "local", "tmp.txt")
	ioutil.WriteFile(filePath2, []byte{1, 2}, os.ModeAppend)

	// Lookup with file name, should find the one we wrote above
	_, err = lookupBin(tmpDir, "tmp.txt")
	require.Nil(err)

	// Write a file under task dir
	filePath3 := filepath.Join(tmpDir, "tmp.txt")
	ioutil.WriteFile(filePath3, []byte{1, 2}, os.ModeAppend)

	// Lookup with file name, should find the one we wrote above
	_, err = lookupBin(tmpDir, "tmp.txt")
	require.Nil(err)

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
	err := os.MkdirAll(filepath.Dir(dst), 666)
	require.NoError(t, err)

	src := filepath.Join(
		"test-resources", "busybox",
		fmt.Sprintf("busybox-%s", runtime.GOARCH),
	)

	err = os.Link(src, dst)
	require.NoError(t, err)
}
