package executor

import (
	"strings"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
)

// dockerIsConnected checks to see if a docker daemon is available (local or remote)
func dockerIsConnected(t *testing.T) bool {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return false
	}

	// Creating a client doesn't actually connect, so make sure we do something
	// like call Version() on it.
	env, err := client.Version()
	if err != nil {
		t.Logf("Failed to connect to docker daemon: %s", err)
		return false
	}

	t.Logf("Successfully connected to docker daemon running version %s", env.Get("Version"))
	return true
}

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
