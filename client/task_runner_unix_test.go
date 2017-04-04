// +build !windows

package client

import (
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

// This test is just to make sure we are resilient to failures when a restart or
// signal is triggered and the task is not running.
func TestTaskRunner_RestartSignalTask_NotRunning(t *testing.T) {
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "100s",
	}

	// Use vault to block the start
	task.Vault = &structs.Vault{Policies: []string{"default"}}

	ctx := testTaskRunnerFromAlloc(t, true, alloc)
	ctx.tr.MarkReceived()
	defer ctx.Cleanup()

	// Control when we get a Vault token
	token := "1234"
	waitCh := make(chan struct{})
	defer close(waitCh)
	handler := func(*structs.Allocation, []string) (map[string]string, error) {
		<-waitCh
		return map[string]string{task.Name: token}, nil
	}
	ctx.tr.vaultClient.(*vaultclient.MockVaultClient).DeriveTokenFn = handler
	go ctx.tr.Run()

	select {
	case <-ctx.tr.WaitCh():
		t.Fatalf("premature exit")
	case <-time.After(1 * time.Second):
	}

	// Send a signal and restart
	if err := ctx.tr.Signal("test", "don't panic", syscall.SIGCHLD); err != nil {
		t.Fatalf("Signalling errored: %v", err)
	}

	// Send a restart
	ctx.tr.Restart("test", "don't panic")

	if len(ctx.upd.events) != 2 {
		t.Fatalf("should have 2 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStatePending {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStatePending)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}
}
