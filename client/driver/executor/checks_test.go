package executor

import (
	"strings"
	"testing"
)

func TestExecScriptCheckNoIsolation(t *testing.T) {
	check := &ExecScriptCheck{
		id:          "foo",
		cmd:         "/bin/echo",
		args:        []string{"hello", "world"},
		taskDir:     "/tmp",
		FSIsolation: false,
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
