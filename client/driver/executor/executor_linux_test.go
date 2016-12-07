package executor

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/client/driver/env"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/nomad/mock"
)

func testExecutorContextWithChroot(t *testing.T) *ExecutorContext {
	taskEnv := env.NewTaskEnvironment(mock.Node())
	task, allocDir := mockAllocDir(t)
	ctx := &ExecutorContext{
		TaskEnv:  taskEnv,
		Task:     task,
		AllocDir: allocDir,
		ChrootEnv: map[string]string{
			"/etc/ld.so.cache":  "/etc/ld.so.cache",
			"/etc/ld.so.conf":   "/etc/ld.so.conf",
			"/etc/ld.so.conf.d": "/etc/ld.so.conf.d",
			"/lib":              "/lib",
			"/lib64":            "/lib64",
			"/usr/lib":          "/usr/lib",
			"/bin/ls":           "/bin/ls",
			"/foobar":           "/does/not/exist",
		},
	}
	return ctx
}

func TestExecutor_IsolationAndConstraints(t *testing.T) {
	testutil.ExecCompatible(t)

	execCmd := ExecCommand{Cmd: "/bin/ls", Args: []string{"-F", "/", "/etc/"}}
	ctx := testExecutorContextWithChroot(t)
	defer ctx.AllocDir.Destroy()

	execCmd.FSIsolation = true
	execCmd.ResourceLimits = true
	execCmd.User = cstructs.DefaultUnpriviledgedUser

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
	file := filepath.Join(ctx.AllocDir.LogDir(), "web.stdout.0")
	output, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("Couldn't read file %v", file)
	}

	act := strings.TrimSpace(string(output))
	if act != expected {
		t.Fatalf("Command output incorrectly: want %v; got %v", expected, act)
	}
}
