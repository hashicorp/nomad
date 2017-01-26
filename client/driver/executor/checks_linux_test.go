package executor

import (
	"log"
	"os"
	"strings"
	"testing"

	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/testutil"
)

func TestExecScriptCheckWithIsolation(t *testing.T) {
	testutil.ExecCompatible(t)

	execCmd := ExecCommand{Cmd: "/bin/echo", Args: []string{"hello world"}}
	ctx, allocDir := testExecutorContextWithChroot(t)
	defer allocDir.Destroy()

	execCmd.FSIsolation = true
	execCmd.ResourceLimits = true
	execCmd.User = dstructs.DefaultUnpriviledgedUser

	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))

	if err := executor.SetContext(ctx); err != nil {
		t.Fatalf("Unexpected error")
	}

	_, err := executor.LaunchCmd(&execCmd)
	if err != nil {
		t.Fatalf("error in launching command: %v", err)
	}

	check := &ExecScriptCheck{
		id:          "foo",
		cmd:         "/bin/echo",
		args:        []string{"hello", "world"},
		taskDir:     ctx.TaskDir,
		FSIsolation: true,
	}

	res := check.Run()
	expectedOutput := "hello world"
	expectedExitCode := 0
	if res.Err != nil {
		t.Fatalf("err: %v", res.Err)
	}
	if strings.TrimSpace(res.Output) != expectedOutput {
		t.Fatalf("output expected: %v, actual: %v", expectedOutput, res.Output)
	}

	if res.ExitCode != expectedExitCode {
		t.Fatalf("exitcode expected: %v, actual: %v", expectedExitCode, res.ExitCode)
	}
}
