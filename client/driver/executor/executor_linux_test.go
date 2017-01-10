package executor

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/nomad/mock"
)

// testExecutorContextWithChroot returns an ExecutorContext and AllocDir with
// chroot. Use testExecutorContext if you don't need a chroot.
//
// The caller is responsible for calling AllocDir.Destroy() to cleanup.
func testExecutorContextWithChroot(t *testing.T) (*ExecutorContext, *allocdir.AllocDir) {
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

	taskEnv := env.NewTaskEnvironment(mock.Node())
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	allocDir := allocdir.NewAllocDir(testLogger(), filepath.Join(os.TempDir(), alloc.ID))
	if err := allocDir.Build(); err != nil {
		log.Fatalf("AllocDir.Build() failed: %v", err)
	}
	if err := allocDir.NewTaskDir(task.Name).Build(chrootEnv, cstructs.FSIsolationChroot); err != nil {
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

func TestExecutor_IsolationAndConstraints(t *testing.T) {
	testutil.ExecCompatible(t)

	execCmd := ExecCommand{Cmd: "/bin/ls", Args: []string{"-F", "/", "/etc/"}}
	ctx, allocDir := testExecutorContextWithChroot(t)
	defer allocDir.Destroy()

	execCmd.FSIsolation = true
	execCmd.ResourceLimits = true
	execCmd.User = dstructs.DefaultUnpriviledgedUser

	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))

	if err := executor.SetContext(ctx); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ps, err := executor.LaunchCmd(&execCmd)
	if err != nil {
		t.Fatalf("error in launching command: %v", err)
	}
	if ps.Pid == 0 {
		t.Fatalf("expected process to start and have non zero pid")
	}
	_, err = executor.Wait()
	if err != nil {
		t.Fatalf("error in waiting for command: %v", err)
	}

	// Check if the resource contraints were applied
	memLimits := filepath.Join(ps.IsolationConfig.CgroupPaths["memory"], "memory.limit_in_bytes")
	data, err := ioutil.ReadFile(memLimits)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expectedMemLim := strconv.Itoa(ctx.Task.Resources.MemoryMB * 1024 * 1024)
	actualMemLim := strings.TrimSpace(string(data))
	if actualMemLim != expectedMemLim {
		t.Fatalf("actual mem limit: %v, expected: %v", string(data), expectedMemLim)
	}

	if err := executor.Exit(); err != nil {
		t.Fatalf("error: %v", err)
	}

	// Check if Nomad has actually removed the cgroups
	if _, err := os.Stat(memLimits); err == nil {
		t.Fatalf("file %v hasn't been removed", memLimits)
	}

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
tmp/
usr/

/etc/:
ld.so.cache
ld.so.conf
ld.so.conf.d/`
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
