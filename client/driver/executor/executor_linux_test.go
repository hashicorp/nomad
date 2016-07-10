package executor

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/testutil"
)

func TestExecutor_IsolationAndConstraints(t *testing.T) {
	testutil.ExecCompatible(t)

	execCmd := ExecCommand{Cmd: "/bin/echo", Args: []string{"hello world"}}
	ctx := testExecutorContext(t)
	defer ctx.AllocDir.Destroy()

	execCmd.FSIsolation = true
	execCmd.ResourceLimits = true
	execCmd.User = cstructs.DefaultUnpriviledgedUser

	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))
	ps, err := executor.LaunchCmd(&execCmd, ctx)
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

	expected := "hello world"
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
