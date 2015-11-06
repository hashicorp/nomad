package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"testing"
)

type nopCloser struct {
	io.ReadWriter
}

func (n *nopCloser) Close() error {
	return nil
}

func TestSpawnDaemon_WriteExitStatus(t *testing.T) {
	// Check if there is python.
	path, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python not detected")
	}

	var b bytes.Buffer
	daemon := &SpawnDaemonCommand{exitFile: &nopCloser{&b}}

	code := 3
	cmd := exec.Command(path, "./test-resources/exiter.py", fmt.Sprintf("%d", code))
	err = cmd.Run()
	actual := daemon.writeExitStatus(err)
	if actual != code {
		t.Fatalf("writeExitStatus(%v) returned %v; want %v", err, actual, code)
	}

	// De-serialize the passed command.
	var exitStatus SpawnExitStatus
	dec := json.NewDecoder(&b)
	if err := dec.Decode(&exitStatus); err != nil {
		t.Fatalf("failed to decode exit status: %v", err)
	}

	if exitStatus.ExitCode != code {
		t.Fatalf("writeExitStatus(%v) wrote exit status %v; want %v", err, exitStatus.ExitCode, code)
	}
}
