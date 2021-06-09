package executor

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/plugins/drivers"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	lconfigs "github.com/opencontainers/runc/libcontainer/configs"
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

	allocDir := allocdir.NewAllocDir(testlog.HCLogger(t), filepath.Join(os.TempDir(), alloc.ID))
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
	t.Parallel()
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
	t.Parallel()
	r := require.New(t)
	testutil.ExecCompatible(t)

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
	data, err := ioutil.ReadFile(memLimits)
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
	t.Parallel()
	require := require.New(t)
	testutil.ExecCompatible(t)

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
		// sanity check that we got some cgroups
		if !strings.Contains(output, ":devices:") {
			return false, fmt.Errorf("was expected cgroup files but found:\n%v", output)
		}
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			// Every cgroup entry should be /nomad/$ALLOC_ID
			if line == "" {
				continue
			}

			// Skip rdma subsystem; rdma was added in most recent kernels and libcontainer/docker
			// don't isolate it by default.
			// :: filters out odd empty cgroup found in latest Ubuntu lines, e.g. 0::/user.slice/user-1000.slice/session-17.scope
			// that is also not used for isolation
			if strings.Contains(line, ":rdma:") || strings.Contains(line, "::") {
				continue
			}

			if !strings.Contains(line, ":/nomad/") {
				return false, fmt.Errorf("Not a member of the alloc's cgroup: expected=...:/nomad/... -- found=%q", line)
			}
		}
		return true, nil
	}, func(err error) { t.Error(err) })
}

// TestExecutor_CgroupPaths asserts that all cgroups created for a task
// are destroyed on shutdown
func TestExecutor_CgroupPathsAreDestroyed(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	testutil.ExecCompatible(t)

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

	var cgroupsPaths string
	tu.WaitForResult(func() (bool, error) {
		output := strings.TrimSpace(testExecCmd.stdout.String())
		// sanity check that we got some cgroups
		if !strings.Contains(output, ":devices:") {
			return false, fmt.Errorf("was expected cgroup files but found:\n%v", output)
		}
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			// Every cgroup entry should be /nomad/$ALLOC_ID
			if line == "" {
				continue
			}

			// Skip rdma subsystem; rdma was added in most recent kernels and libcontainer/docker
			// don't isolate it by default.
			if strings.Contains(line, ":rdma:") || strings.Contains(line, "::") {
				continue
			}

			if !strings.Contains(line, ":/nomad/") {
				return false, fmt.Errorf("Not a member of the alloc's cgroup: expected=...:/nomad/... -- found=%q", line)
			}
		}

		cgroupsPaths = output
		return true, nil
	}, func(err error) { t.Error(err) })

	// shutdown executor and test that cgroups are destroyed
	executor.Shutdown("SIGKILL", 0)

	// test that the cgroup paths are not visible
	tmpFile, err := ioutil.TempFile("", "")
	require.NoError(err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(cgroupsPaths)
	require.NoError(err)
	tmpFile.Close()

	subsystems, err := cgroups.ParseCgroupFile(tmpFile.Name())
	require.NoError(err)

	for subsystem, cgroup := range subsystems {
		if !strings.Contains(cgroup, "nomad/") {
			// this should only be rdma at this point
			continue
		}

		p, err := getCgroupPathHelper(subsystem, cgroup)
		require.NoError(err)
		require.Falsef(cgroups.PathExists(p), "cgroup for %s %s still exists", subsystem, cgroup)
	}
}

func TestUniversalExecutor_LookupTaskBin(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create a temp dir
	tmpDir, err := ioutil.TempDir("", "")
	require.Nil(err)
	defer os.Remove(tmpDir)

	// Create the command
	cmd := &ExecCommand{Env: []string{"PATH=/bin"}, TaskDir: tmpDir}

	// Make a foo subdir
	os.MkdirAll(filepath.Join(tmpDir, "foo"), 0700)

	// Write a file under foo
	filePath := filepath.Join(tmpDir, "foo", "tmp.txt")
	err = ioutil.WriteFile(filePath, []byte{1, 2}, os.ModeAppend)
	require.NoError(err)

	// Lookout with an absolute path to the binary
	cmd.Cmd = "/foo/tmp.txt"
	_, err = lookupTaskBin(cmd)
	require.NoError(err)

	// Write a file under local subdir
	os.MkdirAll(filepath.Join(tmpDir, "local"), 0700)
	filePath2 := filepath.Join(tmpDir, "local", "tmp.txt")
	ioutil.WriteFile(filePath2, []byte{1, 2}, os.ModeAppend)

	// Lookup with file name, should find the one we wrote above
	cmd.Cmd = "tmp.txt"
	_, err = lookupTaskBin(cmd)
	require.NoError(err)

	// Lookup a host absolute path
	cmd.Cmd = "/bin/sh"
	_, err = lookupTaskBin(cmd)
	require.Error(err)
}

// Exec Launch looks for the binary only inside the chroot
func TestExecutor_EscapeContainer(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	testutil.ExecCompatible(t)

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
	t.Parallel()
	testutil.ExecCompatible(t)

	oomPath := "/proc/self/oom_score_adj"
	origValue, err := ioutil.ReadFile(oomPath)
	require.NoError(t, err, "reading oom_score_adj")

	err = ioutil.WriteFile(oomPath, []byte("-100"), 0644)
	require.NoError(t, err, "setting temporary oom_score_adj")

	defer func() {
		err := ioutil.WriteFile(oomPath, origValue, 0644)
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
	t.Parallel()
	testutil.ExecCompatible(t)

	cases := []struct {
		user string
		caps string
	}{
		{
			user: "nobody",
			caps: `
CapInh: 0000000000000000
CapPrm: 0000000000000000
CapEff: 0000000000000000
CapBnd: 0000003fffffdfff
CapAmb: 0000000000000000`,
		},
		{
			user: "root",
			caps: `
CapInh: 0000000000000000
CapPrm: 0000003fffffffff
CapEff: 0000003fffffffff
CapBnd: 0000003fffffffff
CapAmb: 0000000000000000`,
		},
	}

	for _, c := range cases {
		t.Run(c.user, func(t *testing.T) {
			require := require.New(t)

			testExecCmd := testExecutorCommandWithChroot(t)
			execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
			defer allocDir.Destroy()

			execCmd.User = c.user
			execCmd.ResourceLimits = true
			execCmd.Cmd = "/bin/bash"
			execCmd.Args = []string{"-c", "cat /proc/$$/status"}

			executor := NewExecutorWithIsolation(testlog.HCLogger(t))
			defer executor.Shutdown("SIGKILL", 0)

			_, err := executor.Launch(execCmd)
			require.NoError(err)

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

			canonical := func(s string) string {
				s = strings.TrimSpace(s)
				s = regexp.MustCompile("[ \t]+").ReplaceAllString(s, " ")
				s = regexp.MustCompile("[\n\r]+").ReplaceAllString(s, "\n")
				return s
			}

			expected := canonical(c.caps)
			tu.WaitForResult(func() (bool, error) {
				output := canonical(testExecCmd.stdout.String())
				if !strings.Contains(output, expected) {
					return false, fmt.Errorf("capabilities didn't match: want\n%v\n; got:\n%v\n", expected, output)
				}
				return true, nil
			}, func(err error) { require.NoError(err) })
		})
	}

}

func TestExecutor_ClientCleanup(t *testing.T) {
	t.Parallel()
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
	input := []*drivers.DeviceConfig{
		{
			HostPath:    "/dev/null",
			TaskPath:    "/task/dev/null",
			Permissions: "rwm",
		},
	}

	expected := &lconfigs.Device{
		DeviceRule: lconfigs.DeviceRule{
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

// TestUniversalExecutor_NoCgroup asserts that commands are executed in the
// same cgroup as parent process
func TestUniversalExecutor_NoCgroup(t *testing.T) {
	t.Parallel()
	testutil.ExecCompatible(t)

	expectedBytes, err := ioutil.ReadFile("/proc/self/cgroup")
	require.NoError(t, err)

	expected := strings.TrimSpace(string(expectedBytes))

	testExecCmd := testExecutorCommand(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "/bin/cat"
	execCmd.Args = []string{"/proc/self/cgroup"}
	defer allocDir.Destroy()

	execCmd.BasicProcessCgroup = false
	execCmd.ResourceLimits = false

	executor := NewExecutor(testlog.HCLogger(t))
	defer executor.Shutdown("SIGKILL", 0)

	_, err = executor.Launch(execCmd)
	require.NoError(t, err)

	_, err = executor.Wait(context.Background())
	require.NoError(t, err)

	tu.WaitForResult(func() (bool, error) {
		act := strings.TrimSpace(string(testExecCmd.stdout.String()))
		if expected != act {
			return false, fmt.Errorf("expected:\n%s actual:\n%s", expected, act)
		}
		return true, nil
	}, func(err error) {
		stderr := strings.TrimSpace(string(testExecCmd.stderr.String()))
		t.Logf("stderr: %v", stderr)
		require.NoError(t, err)
	})

}
