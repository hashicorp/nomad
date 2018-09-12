package executor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/stats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func init() {
	executorFactories["LibcontainerExecutor"] = libcontainerFactory
}

func libcontainerFactory(l hclog.Logger) Executor {
	return &LibcontainerExecutor{
		id:             strings.Replace(uuid.Generate(), "-", "_", 0),
		logger:         l,
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
	}
}

// testExecutorContextWithChroot returns an ExecutorContext and AllocDir with
// chroot. Use testExecutorContext if you don't need a chroot.
//
// The caller is responsible for calling AllocDir.Destroy() to cleanup.
func testExecutorCommandWithChroot(t *testing.T) (*ExecCommand, *allocdir.AllocDir) {
	chrootEnv := map[string]string{
		"/etc/ld.so.cache":  "/etc/ld.so.cache",
		"/etc/ld.so.conf":   "/etc/ld.so.conf",
		"/etc/ld.so.conf.d": "/etc/ld.so.conf.d",
		"/lib":              "/lib",
		"/lib64":            "/lib64",
		"/usr/lib":          "/usr/lib",
		"/bin/ls":           "/bin/ls",
		"/bin/echo":         "/bin/echo",
		"/bin/bash":         "/bin/bash",
		"/bin/sleep":        "/bin/sleep",
		"/foobar":           "/does/not/exist",
	}

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	taskEnv := env.NewBuilder(mock.Node(), alloc, task, "global").Build()

	allocDir := allocdir.NewAllocDir(testlog.Logger(t), filepath.Join(os.TempDir(), alloc.ID))
	if err := allocDir.Build(); err != nil {
		t.Fatalf("AllocDir.Build() failed: %v", err)
	}
	if err := allocDir.NewTaskDir(task.Name).Build(false, chrootEnv, cstructs.FSIsolationChroot); err != nil {
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

func TestExecutor_IsolationAndConstraints(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	testutil.ExecCompatible(t)

	execCmd, allocDir := testExecutorCommandWithChroot(t)
	execCmd.Cmd = "/bin/ls"
	execCmd.Args = []string{"-F", "/", "/etc/"}
	defer allocDir.Destroy()

	execCmd.FSIsolation = true
	execCmd.ResourceLimits = true
	execCmd.User = dstructs.DefaultUnprivilegedUser

	executor := libcontainerFactory(testLogger(t))
	defer executor.Destroy()

	ps, err := executor.Launch(execCmd)
	require.NoError(err)
	require.NotZero(ps.Pid)

	state, err := executor.Wait()
	require.NoError(err)
	require.Zero(state.ExitCode)

	// Check if the resource constraints were applied
	memLimits := filepath.Join(ps.IsolationConfig.CgroupPaths["memory"], "memory.limit_in_bytes")
	data, err := ioutil.ReadFile(memLimits)
	require.NoError(err)

	expectedMemLim := strconv.Itoa(execCmd.Resources.MemoryMB * 1024 * 1024)
	actualMemLim := strings.TrimSpace(string(data))
	require.Equal(actualMemLim, expectedMemLim)

	require.NoError(executor.Destroy())

	// Check if Nomad has actually removed the cgroups
	_, err = os.Stat(memLimits)
	require.Error(err)

	expected := `/:
alloc/
bin/
dev/
etc/
lib/
lib64/
local/
proc/
secrets/
sys/
tmp/
usr/

/etc/:
ld.so.cache
ld.so.conf
ld.so.conf.d/`
	file := filepath.Join(allocDir.TaskDirs["web"].LogDir, "web.stdout.0")
	stat, err := os.Stat(file)
	require.NoError(err)
	require.NotZero(stat.Sys().(*syscall.Stat_t).Uid)

	tu.WaitForResult(func() (bool, error) {
		output, err := ioutil.ReadFile(file)
		if err != nil {
			return false, err
		}
		act := strings.TrimSpace(string(output))
		if act != expected {
			return false, fmt.Errorf("Command output incorrectly: want %v; got %v", expected, act)
		}
		return true, nil
	}, func(err error) { t.Error(err) })
}

func TestExecutor_ClientCleanup(t *testing.T) {
	t.Parallel()
	testutil.ExecCompatible(t)
	require := require.New(t)

	execCmd, allocDir := testExecutorCommandWithChroot(t)
	defer allocDir.Destroy()

	executor := libcontainerFactory(testLogger(t))
	defer executor.Destroy()

	// Need to run a command which will produce continuous output but not
	// too quickly to ensure executor.Exit() stops the process.
	execCmd.Cmd = "/bin/bash"
	execCmd.Args = []string{"-c", "while true; do /bin/echo X; /bin/sleep 1; done"}
	execCmd.FSIsolation = true
	execCmd.ResourceLimits = true
	execCmd.User = "nobody"

	ps, err := executor.Launch(execCmd)
	require.NoError(err)
	require.NotZero(ps.Pid)
	time.Sleep(500 * time.Millisecond)
	require.NoError(executor.Destroy())

	file := filepath.Join(allocDir.TaskDirs["web"].LogDir, "web.stdout.0")
	finfo, err := os.Stat(file)
	require.NoError(err)
	require.NotZero(finfo.Size())
	time.Sleep(2 * time.Second)
	finfo1, err := os.Stat(file)
	require.NoError(err)
	require.Equal(finfo.Size(), finfo1.Size())
}
