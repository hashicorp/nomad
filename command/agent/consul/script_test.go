package consul

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

// blockingScriptExec implements ScriptExec by running a subcommand that never
// exits.
type blockingScriptExec struct {
	// running is ticked before blocking to allow synchronizing operations
	running chan struct{}

	// set to true if Exec is called and has exited
	exited bool
}

func newBlockingScriptExec() *blockingScriptExec {
	return &blockingScriptExec{running: make(chan struct{})}
}

func (b *blockingScriptExec) Exec(ctx context.Context, _ string, _ []string) ([]byte, int, error) {
	b.running <- mark
	cmd := exec.CommandContext(ctx, "/bin/sleep", "9000")
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		if !exitErr.Success() {
			code = 1
		}
	}
	b.exited = true
	return []byte{}, code, err
}

// TestConsulScript_Exec_Cancel asserts cancelling a script check shortcircuits
// any running scripts.
func TestConsulScript_Exec_Cancel(t *testing.T) {
	serviceCheck := structs.ServiceCheck{
		Name:     "sleeper",
		Interval: time.Hour,
		Timeout:  time.Hour,
	}
	exec := newBlockingScriptExec()

	// pass nil for heartbeater as it shouldn't be called
	check := newScriptCheck("checkid", &serviceCheck, exec, nil, testLogger(), nil)
	handle := check.run()

	// wait until Exec is called
	<-exec.running

	// cancel now that we're blocked in exec
	handle.cancel()

	select {
	case <-handle.wait():
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}
	if !exec.exited {
		t.Errorf("expected script executor to run and exit but it has not")
	}
}

type fakeHeartbeater struct {
	updates chan string
}

func (f *fakeHeartbeater) UpdateTTL(checkID, output, status string) error {
	f.updates <- status
	return nil
}

func newFakeHeartbeater() *fakeHeartbeater {
	return &fakeHeartbeater{updates: make(chan string)}
}

// TestConsulScript_Exec_Timeout asserts a script will be killed when the
// timeout is reached.
func TestConsulScript_Exec_Timeout(t *testing.T) {
	t.Parallel() // run the slow tests in parallel
	serviceCheck := structs.ServiceCheck{
		Name:     "sleeper",
		Interval: time.Hour,
		Timeout:  time.Second,
	}
	exec := newBlockingScriptExec()

	hb := newFakeHeartbeater()
	check := newScriptCheck("checkid", &serviceCheck, exec, hb, testLogger(), nil)
	handle := check.run()
	defer handle.cancel() // just-in-case cleanup
	<-exec.running

	// Check for UpdateTTL call
	select {
	case update := <-hb.updates:
		if update != api.HealthCritical {
			t.Error("expected %q due to timeout but received %q", api.HealthCritical, update)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}
	if !exec.exited {
		t.Errorf("expected script executor to run and exit but it has not")
	}

	// Cancel and watch for exit
	handle.cancel()
	select {
	case <-handle.wait():
		// ok!
	case update := <-hb.updates:
		t.Errorf("unexpected UpdateTTL call on exit with status=%q", update)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}
}

type noopExec struct{}

func (noopExec) Exec(context.Context, string, []string) ([]byte, int, error) {
	return []byte{}, 0, nil
}

// TestConsulScript_Exec_Shutdown asserts a script will be executed once more
// when told to shutdown.
func TestConsulScript_Exec_Shutdown(t *testing.T) {
	serviceCheck := structs.ServiceCheck{
		Name:     "sleeper",
		Interval: time.Hour,
		Timeout:  3 * time.Second,
	}

	hb := newFakeHeartbeater()
	shutdown := make(chan struct{})
	check := newScriptCheck("checkid", &serviceCheck, noopExec{}, hb, testLogger(), shutdown)
	handle := check.run()
	defer handle.cancel() // just-in-case cleanup

	// Tell scriptCheck to exit
	close(shutdown)

	select {
	case update := <-hb.updates:
		if update != api.HealthPassing {
			t.Error("expected %q due to timeout but received %q", api.HealthPassing, update)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}

	select {
	case <-handle.wait():
		// ok!
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}
}
