// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/drivers/shared/capabilities"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/plugins/drivers"
	tu "github.com/hashicorp/nomad/testutil"
	lconfigs "github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func init() {
	executorFactories["LibcontainerExecutor"] = libcontainerFactory
}

var libcontainerFactory = executorFactory{
	new: NewExecutorWithIsolation,
	configureExecCmd: func(t *testing.T, cmd *ExecCommand) {
		cmd.ResourceLimits = true
		setupRootfs(t, cmd.TaskDir)
	},
}

// testExecutorContextWithChroot returns an ExecutorContext and AllocDir with
// chroot. Use testExecutorContext if you don't need a chroot.
//
// The caller is responsible for calling AllocDir.Destroy() to cleanup.
func testExecutorCommandWithChroot(t *testing.T) *testExecCmd {
	chrootEnv := map[string]string{
		"/etc/ld.so.cache":  "/etc/ld.so.cache",
		"/etc/ld.so.conf":   "/etc/ld.so.conf",
		"/etc/ld.so.conf.d": "/etc/ld.so.conf.d",
		"/etc/passwd":       "/etc/passwd",
		"/lib":              "/lib",
		"/lib64":            "/lib64",
		"/usr/lib":          "/usr/lib",
		"/bin/ls":           "/bin/ls",
		"/bin/cat":          "/bin/cat",
		"/bin/echo":         "/bin/echo",
		"/bin/bash":         "/bin/bash",
		"/bin/sleep":        "/bin/sleep",
		"/foobar":           "/does/not/exist",
	}

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	taskEnv := taskenv.NewBuilder(mock.Node(), alloc, task, "global").Build()

	allocDir := allocdir.NewAllocDir(testlog.HCLogger(t), os.TempDir(), alloc.ID)
	if err := allocDir.Build(); err != nil {
		t.Fatalf("AllocDir.Build() failed: %v", err)
	}
	if err := allocDir.NewTaskDir(task.Name).Build(true, chrootEnv); err != nil {
		allocDir.Destroy()
		t.Fatalf("allocDir.NewTaskDir(%q) failed: %v", task.Name, err)
	}
	td := allocDir.TaskDirs[task.Name]
	cmd := &ExecCommand{
		Env:     taskEnv.List(),
		TaskDir: td.Dir,
		Resources: &drivers.Resources{
			NomadResources: alloc.AllocatedResources.Tasks[task.Name],
			LinuxResources: &drivers.LinuxResources{
				CpusetCgroupPath: cgroupslib.LinuxResourcesPath(alloc.ID, task.Name),
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

func TestExecutor_configureNamespaces(t *testing.T) {
	ci.Parallel(t)
	t.Run("host host", func(t *testing.T) {
		require.Equal(t, lconfigs.Namespaces{
			{Type: lconfigs.NEWNS},
		}, configureNamespaces("host", "host"))
	})

	t.Run("host private", func(t *testing.T) {
		require.Equal(t, lconfigs.Namespaces{
			{Type: lconfigs.NEWNS},
			{Type: lconfigs.NEWIPC},
		}, configureNamespaces("host", "private"))
	})

	t.Run("private host", func(t *testing.T) {
		require.Equal(t, lconfigs.Namespaces{
			{Type: lconfigs.NEWNS},
			{Type: lconfigs.NEWPID},
		}, configureNamespaces("private", "host"))
	})

	t.Run("private private", func(t *testing.T) {
		require.Equal(t, lconfigs.Namespaces{
			{Type: lconfigs.NEWNS},
			{Type: lconfigs.NEWPID},
			{Type: lconfigs.NEWIPC},
		}, configureNamespaces("private", "private"))
	})
}

func TestExecutor_Isolation_PID_and_IPC_hostMode(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)
	testutil.ExecCompatible(t)

	testExecCmd := testExecutorCommandWithChroot(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "/bin/ls"
	execCmd.Args = []string{"-F", "/", "/etc/"}
	defer allocDir.Destroy()

	execCmd.ResourceLimits = true
	execCmd.ModePID = "host" // disable PID namespace
	execCmd.ModeIPC = "host" // disable IPC namespace

	executor := NewExecutorWithIsolation(testlog.HCLogger(t))
	defer executor.Shutdown("SIGKILL", 0)

	ps, err := executor.Launch(execCmd)
	r.NoError(err)
	r.NotZero(ps.Pid)

	estate, err := executor.Wait(context.Background())
	r.NoError(err)
	r.Zero(estate.ExitCode)

	lexec, ok := executor.(*LibcontainerExecutor)
	r.True(ok)

	// Check that namespaces were applied to the container config
	config := lexec.container.Config()

	r.Contains(config.Namespaces, lconfigs.Namespace{Type: lconfigs.NEWNS})
	r.NotContains(config.Namespaces, lconfigs.Namespace{Type: lconfigs.NEWPID})
	r.NotContains(config.Namespaces, lconfigs.Namespace{Type: lconfigs.NEWIPC})

	// Shut down executor
	r.NoError(executor.Shutdown("", 0))
	executor.Wait(context.Background())
}

func TestExecutor_IsolationAndConstraints(t *testing.T) {
	ci.Parallel(t)
	testutil.ExecCompatible(t)
	testutil.CgroupsCompatibleV1(t) // todo(shoenig): hard codes cgroups v1 lookup

	r := require.New(t)

	testExecCmd := testExecutorCommandWithChroot(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "/bin/ls"
	execCmd.Args = []string{"-F", "/", "/etc/"}
	defer allocDir.Destroy()

	execCmd.ResourceLimits = true
	execCmd.ModePID = "private"
	execCmd.ModeIPC = "private"

	executor := NewExecutorWithIsolation(testlog.HCLogger(t))
	defer executor.Shutdown("SIGKILL", 0)

	ps, err := executor.Launch(execCmd)
	r.NoError(err)
	r.NotZero(ps.Pid)

	estate, err := executor.Wait(context.Background())
	r.NoError(err)
	r.Zero(estate.ExitCode)

	lexec, ok := executor.(*LibcontainerExecutor)
	r.True(ok)

	// Check if the resource constraints were applied
	state, err := lexec.container.State()
	r.NoError(err)

	memLimits := filepath.Join(state.CgroupPaths["memory"], "memory.limit_in_bytes")
	data, err := os.ReadFile(memLimits)
	r.NoError(err)

	expectedMemLim := strconv.Itoa(int(execCmd.Resources.NomadResources.Memory.MemoryMB * 1024 * 1024))
	actualMemLim := strings.TrimSpace(string(data))
	r.Equal(actualMemLim, expectedMemLim)

	// Check that namespaces were applied to the container config
	config := lexec.container.Config()

	r.Contains(config.Namespaces, lconfigs.Namespace{Type: lconfigs.NEWNS})
	r.Contains(config.Namespaces, lconfigs.Namespace{Type: lconfigs.NEWPID})
	r.Contains(config.Namespaces, lconfigs.Namespace{Type: lconfigs.NEWIPC})

	// Shut down executor
	r.NoError(executor.Shutdown("", 0))
	executor.Wait(context.Background())

	// Check if Nomad has actually removed the cgroups
	tu.WaitForResult(func() (bool, error) {
		_, err = os.Stat(memLimits)
		if err == nil {
			return false, fmt.Errorf("expected an error from os.Stat %s", memLimits)
		}
		return true, nil
	}, func(err error) { t.Error(err) })

	expected := `/:
alloc/
bin/
dev/
etc/
lib/
lib64/
local/
private/
proc/
secrets/
sys/
tmp/
usr/

/etc/:
ld.so.cache
ld.so.conf
ld.so.conf.d/
passwd`
	tu.WaitForResult(func() (bool, error) {
		output := testExecCmd.stdout.String()
		act := strings.TrimSpace(string(output))
		if act != expected {
			return false, fmt.Errorf("Command output incorrectly: want %v; got %v", expected, act)
		}
		return true, nil
	}, func(err error) { t.Error(err) })
}

// TestExecutor_CgroupPaths asserts that process starts with independent cgroups
// hierarchy created for this process
func TestExecutor_CgroupPaths(t *testing.T) {
	ci.Parallel(t)
	testutil.ExecCompatible(t)

	require := require.New(t)

	testExecCmd := testExecutorCommandWithChroot(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "/bin/bash"
	execCmd.Args = []string{"-c", "sleep 0.2; cat /proc/self/cgroup"}
	defer allocDir.Destroy()

	execCmd.ResourceLimits = true

	executor := NewExecutorWithIsolation(testlog.HCLogger(t))
	defer executor.Shutdown("SIGKILL", 0)

	ps, err := executor.Launch(execCmd)
	require.NoError(err)
	require.NotZero(ps.Pid)

	state, err := executor.Wait(context.Background())
	require.NoError(err)
	require.Zero(state.ExitCode)

	tu.WaitForResult(func() (bool, error) {
		output := strings.TrimSpace(testExecCmd.stdout.String())
		switch cgroupslib.GetMode() {
		case cgroupslib.CG2:
			isScope := strings.HasSuffix(output, ".scope")
			require.True(isScope)
		default:
			// Verify that we got some cgroups
			if !strings.Contains(output, ":devices:") {
				return false, fmt.Errorf("was expected cgroup files but found:\n%v", output)
			}
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				// Every cgroup entry should be /nomad/$ALLOC_ID
				if line == "" {
					continue
				}

				// Skip rdma & misc subsystem; rdma was added in most recent kernels and libcontainer/docker
				// don't isolate it by default.
				// :: filters out odd empty cgroup found in latest Ubuntu lines, e.g. 0::/user.slice/user-1000.slice/session-17.scope
				// that is also not used for isolation
				if strings.Contains(line, ":rdma:") || strings.Contains(line, ":misc:") || strings.Contains(line, "::") {
					continue
				}
				if !strings.Contains(line, ":/nomad/") {
					return false, fmt.Errorf("Not a member of the alloc's cgroup: expected=...:/nomad/... -- found=%q", line)
				}

			}
		}
		return true, nil
	}, func(err error) { t.Error(err) })
}

func TestExecutor_LookupTaskBin(t *testing.T) {
	ci.Parallel(t)

	// Create a temp dir
	taskDir := t.TempDir()
	mountDir := t.TempDir()

	// Create the command with mounts
	cmd := &ExecCommand{
		Env:     []string{"PATH=/bin"},
		TaskDir: taskDir,
		Mounts:  []*drivers.MountConfig{{TaskPath: "/srv", HostPath: mountDir}},
	}

	// Make a /foo /local/foo and /usr/local/bin subdirs under task dir
	// and /bar under mountdir
	must.NoError(t, os.MkdirAll(filepath.Join(taskDir, "foo"), 0700))
	must.NoError(t, os.MkdirAll(filepath.Join(taskDir, "local/foo"), 0700))
	must.NoError(t, os.MkdirAll(filepath.Join(taskDir, "usr/local/bin"), 0700))
	must.NoError(t, os.MkdirAll(filepath.Join(mountDir, "bar"), 0700))

	writeFile := func(paths ...string) {
		t.Helper()
		path := filepath.Join(paths...)
		must.NoError(t, os.WriteFile(path, []byte("hello"), 0o700))
	}

	// Write some files
	writeFile(taskDir, "usr/local/bin", "tmp0.txt") // under /usr/local/bin in taskdir
	writeFile(taskDir, "foo", "tmp1.txt")           // under foo in taskdir
	writeFile(taskDir, "local", "tmp2.txt")         // under root of task-local dir
	writeFile(taskDir, "local/foo", "tmp3.txt")     // under foo in task-local dir
	writeFile(mountDir, "tmp4.txt")                 // under root of mount dir
	writeFile(mountDir, "bar/tmp5.txt")             // under bar in mount dir

	testCases := []struct {
		name           string
		cmd            string
		expectErr      string
		expectTaskPath string
		expectHostPath string
	}{
		{
			name:           "lookup with file name in PATH",
			cmd:            "tmp0.txt",
			expectTaskPath: "/usr/local/bin/tmp0.txt",
			expectHostPath: filepath.Join(taskDir, "usr/local/bin/tmp0.txt"),
		},
		{
			name:           "lookup with absolute path to binary",
			cmd:            "/foo/tmp1.txt",
			expectTaskPath: "/foo/tmp1.txt",
			expectHostPath: filepath.Join(taskDir, "foo/tmp1.txt"),
		},
		{
			name:           "lookup in task local dir with absolute path to binary",
			cmd:            "/local/tmp2.txt",
			expectTaskPath: "/local/tmp2.txt",
			expectHostPath: filepath.Join(taskDir, "local/tmp2.txt"),
		},
		{
			name:           "lookup in task local dir with relative path to binary",
			cmd:            "local/tmp2.txt",
			expectTaskPath: "/local/tmp2.txt",
			expectHostPath: filepath.Join(taskDir, "local/tmp2.txt"),
		},
		{
			name:           "lookup in task local dir with file name",
			cmd:            "tmp2.txt",
			expectTaskPath: "/local/tmp2.txt",
			expectHostPath: filepath.Join(taskDir, "local/tmp2.txt"),
		},
		{
			name:           "lookup in task local subdir with absolute path to binary",
			cmd:            "/local/foo/tmp3.txt",
			expectTaskPath: "/local/foo/tmp3.txt",
			expectHostPath: filepath.Join(taskDir, "local/foo/tmp3.txt"),
		},
		{
			name:      "lookup host absolute path outside taskdir",
			cmd:       "/bin/sh",
			expectErr: "file /bin/sh not found under path " + taskDir,
		},
		{
			name:           "lookup file from mount with absolute path",
			cmd:            "/srv/tmp4.txt",
			expectTaskPath: "/srv/tmp4.txt",
			expectHostPath: filepath.Join(mountDir, "tmp4.txt"),
		},
		{
			name:      "lookup file from mount with file name fails",
			cmd:       "tmp4.txt",
			expectErr: "file tmp4.txt not found under path",
		},
		{
			name:           "lookup file from mount with subdir",
			cmd:            "/srv/bar/tmp5.txt",
			expectTaskPath: "/srv/bar/tmp5.txt",
			expectHostPath: filepath.Join(mountDir, "bar/tmp5.txt"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd.Cmd = tc.cmd
			taskPath, hostPath, err := lookupTaskBin(cmd)
			if tc.expectErr == "" {
				must.NoError(t, err)
				test.Eq(t, tc.expectTaskPath, taskPath)
				test.Eq(t, tc.expectHostPath, hostPath)
			} else {
				test.EqError(t, err, tc.expectErr)
			}
		})
	}
}

// Exec Launch looks for the binary only inside the chroot
func TestExecutor_EscapeContainer(t *testing.T) {
	ci.Parallel(t)
	testutil.ExecCompatible(t)
	testutil.CgroupsCompatibleV1(t) // todo(shoenig) kills the terminal, probably defaulting to /

	require := require.New(t)

	testExecCmd := testExecutorCommandWithChroot(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "/bin/kill" // missing from the chroot container
	defer allocDir.Destroy()

	execCmd.ResourceLimits = true

	executor := NewExecutorWithIsolation(testlog.HCLogger(t))
	defer executor.Shutdown("SIGKILL", 0)

	_, err := executor.Launch(execCmd)
	require.Error(err)
	require.Regexp("^file /bin/kill not found under path", err)

	// Bare files are looked up using the system path, inside the container
	allocDir.Destroy()
	testExecCmd = testExecutorCommandWithChroot(t)
	execCmd, allocDir = testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "kill"
	_, err = executor.Launch(execCmd)
	require.Error(err)
	require.Regexp("^file kill not found under path", err)

	allocDir.Destroy()
	testExecCmd = testExecutorCommandWithChroot(t)
	execCmd, allocDir = testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "echo"
	_, err = executor.Launch(execCmd)
	require.NoError(err)
}

// TestExecutor_DoesNotInheritOomScoreAdj asserts that the exec processes do not
// inherit the oom_score_adj value of Nomad agent/executor process
func TestExecutor_DoesNotInheritOomScoreAdj(t *testing.T) {
	ci.Parallel(t)
	testutil.ExecCompatible(t)

	oomPath := "/proc/self/oom_score_adj"
	origValue, err := os.ReadFile(oomPath)
	require.NoError(t, err, "reading oom_score_adj")

	err = os.WriteFile(oomPath, []byte("-100"), 0644)
	require.NoError(t, err, "setting temporary oom_score_adj")

	defer func() {
		err := os.WriteFile(oomPath, origValue, 0644)
		require.NoError(t, err, "restoring oom_score_adj")
	}()

	testExecCmd := testExecutorCommandWithChroot(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	defer allocDir.Destroy()

	execCmd.ResourceLimits = true
	execCmd.Cmd = "/bin/bash"
	execCmd.Args = []string{"-c", "cat /proc/self/oom_score_adj"}

	executor := NewExecutorWithIsolation(testlog.HCLogger(t))
	defer executor.Shutdown("SIGKILL", 0)

	_, err = executor.Launch(execCmd)
	require.NoError(t, err)

	ch := make(chan interface{})
	go func() {
		executor.Wait(context.Background())
		close(ch)
	}()

	select {
	case <-ch:
		// all good
	case <-time.After(5 * time.Second):
		require.Fail(t, "timeout waiting for exec to shutdown")
	}

	expected := "0"
	tu.WaitForResult(func() (bool, error) {
		output := strings.TrimSpace(testExecCmd.stdout.String())
		if output != expected {
			return false, fmt.Errorf("oom_score_adj didn't match: want\n%v\n; got:\n%v\n", expected, output)
		}
		return true, nil
	}, func(err error) { require.NoError(t, err) })

}

func TestExecutor_Capabilities(t *testing.T) {
	ci.Parallel(t)
	testutil.ExecCompatible(t)

	cases := []struct {
		user         string
		capAdd       []string
		capDrop      []string
		capsExpected string
	}{
		{
			user: "nobody",
			capsExpected: `
CapInh: 00000000a80405fb
CapPrm: 00000000a80405fb
CapEff: 00000000a80405fb
CapBnd: 00000000a80405fb
CapAmb: 00000000a80405fb`,
		},
		{
			user: "root",
			capsExpected: `
CapInh: 0000000000000000
CapPrm: 0000003fffffffff
CapEff: 0000003fffffffff
CapBnd: 0000003fffffffff
CapAmb: 0000000000000000`,
		},
		{
			user:    "nobody",
			capDrop: []string{"all"},
			capAdd:  []string{"net_bind_service"},
			capsExpected: `
CapInh: 0000000000000400
CapPrm: 0000000000000400
CapEff: 0000000000000400
CapBnd: 0000000000000400
CapAmb: 0000000000000400`,
		},
	}

	for _, c := range cases {
		t.Run(c.user, func(t *testing.T) {

			testExecCmd := testExecutorCommandWithChroot(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			defer allocDir.Destroy()

			execCmd.User = c.user
			execCmd.ResourceLimits = true
			execCmd.Cmd = "/bin/bash"
			execCmd.Args = []string{"-c", "cat /proc/$$/status"}

			capsBasis := capabilities.NomadDefaults()
			capsAllowed := capsBasis.Slice(true)
			if c.capDrop != nil || c.capAdd != nil {
				calcCaps, err := capabilities.Calculate(
					capsBasis, capsAllowed, c.capAdd, c.capDrop)
				require.NoError(t, err)
				execCmd.Capabilities = calcCaps
			} else {
				execCmd.Capabilities = capsAllowed
			}

			executor := NewExecutorWithIsolation(testlog.HCLogger(t))
			defer executor.Shutdown("SIGKILL", 0)

			_, err := executor.Launch(execCmd)
			require.NoError(t, err)

			ch := make(chan interface{})
			go func() {
				executor.Wait(context.Background())
				close(ch)
			}()

			select {
			case <-ch:
				// all good
			case <-time.After(5 * time.Second):
				require.Fail(t, "timeout waiting for exec to shutdown")
			}

			canonical := func(s string) string {
				s = strings.TrimSpace(s)
				s = regexp.MustCompile("[ \t]+").ReplaceAllString(s, " ")
				s = regexp.MustCompile("[\n\r]+").ReplaceAllString(s, "\n")
				return s
			}

			expected := canonical(c.capsExpected)
			tu.WaitForResult(func() (bool, error) {
				output := canonical(testExecCmd.stdout.String())
				if !strings.Contains(output, expected) {
					return false, fmt.Errorf("capabilities didn't match: want\n%v\n; got:\n%v\n", expected, output)
				}
				return true, nil
			}, func(err error) { require.NoError(t, err) })
		})
	}

}

func TestExecutor_ClientCleanup(t *testing.T) {
	ci.Parallel(t)
	testutil.ExecCompatible(t)
	require := require.New(t)

	testExecCmd := testExecutorCommandWithChroot(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	defer allocDir.Destroy()

	executor := NewExecutorWithIsolation(testlog.HCLogger(t))
	defer executor.Shutdown("", 0)

	// Need to run a command which will produce continuous output but not
	// too quickly to ensure executor.Exit() stops the process.
	execCmd.Cmd = "/bin/bash"
	execCmd.Args = []string{"-c", "while true; do /bin/echo X; /bin/sleep 1; done"}
	execCmd.ResourceLimits = true

	ps, err := executor.Launch(execCmd)

	require.NoError(err)
	require.NotZero(ps.Pid)
	time.Sleep(500 * time.Millisecond)
	require.NoError(executor.Shutdown("SIGINT", 100*time.Millisecond))

	ch := make(chan interface{})
	go func() {
		executor.Wait(context.Background())
		close(ch)
	}()

	select {
	case <-ch:
		// all good
	case <-time.After(5 * time.Second):
		require.Fail("timeout waiting for exec to shutdown")
	}

	output := testExecCmd.stdout.String()
	require.NotZero(len(output))
	time.Sleep(2 * time.Second)
	output1 := testExecCmd.stdout.String()
	require.Equal(len(output), len(output1))
}

func TestExecutor_cmdDevices(t *testing.T) {
	ci.Parallel(t)
	input := []*drivers.DeviceConfig{
		{
			HostPath:    "/dev/null",
			TaskPath:    "/task/dev/null",
			Permissions: "rwm",
		},
	}

	expected := &devices.Device{
		Rule: devices.Rule{
			Type:        99,
			Major:       1,
			Minor:       3,
			Permissions: "rwm",
		},
		Path: "/task/dev/null",
	}

	found, err := cmdDevices(input)
	require.NoError(t, err)
	require.Len(t, found, 1)

	// ignore file permission and ownership
	// as they are host specific potentially
	d := found[0]
	d.FileMode = 0
	d.Uid = 0
	d.Gid = 0

	require.EqualValues(t, expected, d)
}

func TestExecutor_cmdMounts(t *testing.T) {
	ci.Parallel(t)
	input := []*drivers.MountConfig{
		{
			HostPath: "/host/path-ro",
			TaskPath: "/task/path-ro",
			Readonly: true,
		},
		{
			HostPath: "/host/path-rw",
			TaskPath: "/task/path-rw",
			Readonly: false,
		},
	}

	expected := []*lconfigs.Mount{
		{
			Source:           "/host/path-ro",
			Destination:      "/task/path-ro",
			Flags:            unix.MS_BIND | unix.MS_RDONLY,
			Device:           "bind",
			PropagationFlags: []int{unix.MS_PRIVATE | unix.MS_REC},
		},
		{
			Source:           "/host/path-rw",
			Destination:      "/task/path-rw",
			Flags:            unix.MS_BIND,
			Device:           "bind",
			PropagationFlags: []int{unix.MS_PRIVATE | unix.MS_REC},
		},
	}

	require.EqualValues(t, expected, cmdMounts(input))
}
