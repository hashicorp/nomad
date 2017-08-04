package consul

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestMain(m *testing.M) {
	if !testtask.Run() {
		os.Exit(m.Run())
	}
}

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
	b.running <- struct{}{}
	cmd := exec.CommandContext(ctx, testtask.Path(), "sleep", "9000h")
	testtask.SetCmdEnv(cmd)
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
	check := newScriptCheck("allocid", "testtask", "checkid", &serviceCheck, exec, nil, testLogger(), nil)
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

type execStatus struct {
	checkID string
	output  string
	status  string
}

// fakeHeartbeater implements the heartbeater interface to allow mocking out
// Consul in script executor tests.
type fakeHeartbeater struct {
	updates chan execStatus
}

func (f *fakeHeartbeater) UpdateTTL(checkID, output, status string) error {
	f.updates <- execStatus{checkID: checkID, output: output, status: status}
	return nil
}

func newFakeHeartbeater() *fakeHeartbeater {
	return &fakeHeartbeater{updates: make(chan execStatus)}
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
	check := newScriptCheck("allocid", "testtask", "checkid", &serviceCheck, exec, hb, testLogger(), nil)
	handle := check.run()
	defer handle.cancel() // just-in-case cleanup
	<-exec.running

	// Check for UpdateTTL call
	select {
	case update := <-hb.updates:
		if update.status != api.HealthCritical {
			t.Errorf("expected %q due to timeout but received %q", api.HealthCritical, update)
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

// sleeperExec sleeps for 100ms but returns successfully to allow testing timeout conditions
type sleeperExec struct{}

func (sleeperExec) Exec(context.Context, string, []string) ([]byte, int, error) {
	time.Sleep(100 * time.Millisecond)
	return []byte{}, 0, nil
}

// TestConsulScript_Exec_TimeoutCritical asserts a script will be killed when
// the timeout is reached and always set a critical status regardless of what
// Exec returns.
func TestConsulScript_Exec_TimeoutCritical(t *testing.T) {
	t.Parallel() // run the slow tests in parallel
	serviceCheck := structs.ServiceCheck{
		Name:     "sleeper",
		Interval: time.Hour,
		Timeout:  time.Nanosecond,
	}
	hb := newFakeHeartbeater()
	check := newScriptCheck("allocid", "testtask", "checkid", &serviceCheck, sleeperExec{}, hb, testLogger(), nil)
	handle := check.run()
	defer handle.cancel() // just-in-case cleanup

	// Check for UpdateTTL call
	select {
	case update := <-hb.updates:
		if update.status != api.HealthCritical {
			t.Errorf("expected %q due to timeout but received %q", api.HealthCritical, update)
		}
		if update.output != context.DeadlineExceeded.Error() {
			t.Errorf("expected output=%q but found: %q", context.DeadlineExceeded.Error(), update.output)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to timeout")
	}
}

// simpleExec is a fake ScriptExecutor that returns whatever is specified.
type simpleExec struct {
	code int
	err  error
}

func (s simpleExec) Exec(context.Context, string, []string) ([]byte, int, error) {
	return []byte(fmt.Sprintf("code=%d err=%v", s.code, s.err)), s.code, s.err
}

// newSimpleExec creates a new ScriptExecutor that returns the given code and err.
func newSimpleExec(code int, err error) simpleExec {
	return simpleExec{code: code, err: err}
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
	exec := newSimpleExec(0, nil)
	check := newScriptCheck("allocid", "testtask", "checkid", &serviceCheck, exec, hb, testLogger(), shutdown)
	handle := check.run()
	defer handle.cancel() // just-in-case cleanup

	// Tell scriptCheck to exit
	close(shutdown)

	select {
	case update := <-hb.updates:
		if update.status != api.HealthPassing {
			t.Errorf("expected %q due to timeout but received %q", api.HealthCritical, update)
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

func TestConsulScript_Exec_Codes(t *testing.T) {
	run := func(code int, err error, expected string) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			serviceCheck := structs.ServiceCheck{
				Name:     "test",
				Interval: time.Hour,
				Timeout:  3 * time.Second,
			}

			hb := newFakeHeartbeater()
			shutdown := make(chan struct{})
			exec := newSimpleExec(code, err)
			check := newScriptCheck("allocid", "testtask", "checkid", &serviceCheck, exec, hb, testLogger(), shutdown)
			handle := check.run()
			defer handle.cancel()

			select {
			case update := <-hb.updates:
				if update.status != expected {
					t.Errorf("expected %q but received %q", expected, update)
				}
				// assert output is being reported
				expectedOutput := fmt.Sprintf("code=%d err=%v", code, err)
				if err != nil {
					expectedOutput = err.Error()
				}
				if update.output != expectedOutput {
					t.Errorf("expected output=%q but found: %q", expectedOutput, update.output)
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("timed out waiting for script check to exec")
			}
		}
	}

	// Test exit codes with errors
	t.Run("Passing", run(0, nil, api.HealthPassing))
	t.Run("Warning", run(1, nil, api.HealthWarning))
	t.Run("Critical-2", run(2, nil, api.HealthCritical))
	t.Run("Critical-9000", run(9000, nil, api.HealthCritical))

	// Errors should always cause Critical status
	err := fmt.Errorf("test error")
	t.Run("Error-0", run(0, err, api.HealthCritical))
	t.Run("Error-1", run(1, err, api.HealthCritical))
	t.Run("Error-2", run(2, err, api.HealthCritical))
	t.Run("Error-9000", run(9000, err, api.HealthCritical))
}
