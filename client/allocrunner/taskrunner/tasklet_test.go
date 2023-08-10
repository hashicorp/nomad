// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	if !testtask.Run() {
		os.Exit(m.Run())
	}
}

func TestTasklet_Exec_HappyPath(t *testing.T) {
	ci.Parallel(t)

	results := []execResult{
		{[]byte("output"), 0, nil},
		{[]byte("output"), 1, nil},
		{[]byte("output"), 0, context.DeadlineExceeded},
		{[]byte("<ignored output>"), 2, fmt.Errorf("some error")},
		{[]byte("error9000"), 9000, nil},
	}
	exec := newScriptedExec(results)
	tm := newTaskletMock(exec, testlog.HCLogger(t), time.Nanosecond, 3*time.Second)

	handle := tm.run()
	defer handle.cancel() // just-in-case cleanup

	deadline := time.After(3 * time.Second)
	for i := 0; i <= 4; i++ {
		select {
		case result := <-tm.calls:
			// for the happy path without cancelations or shutdowns, we expect
			// to get the results passed to the callback in order and without
			// modification
			assert.Equal(t, result, results[i])
		case <-deadline:
			t.Fatalf("timed out waiting for all script checks to finish")
		}
	}
}

// TestTasklet_Exec_Cancel asserts cancelling a tasklet short-circuits
// any running executions the tasklet
func TestTasklet_Exec_Cancel(t *testing.T) {
	ci.Parallel(t)

	exec, cancel := newBlockingScriptExec()
	defer cancel()
	tm := newTaskletMock(exec, testlog.HCLogger(t), time.Hour, time.Hour)

	handle := tm.run()
	<-exec.running  // wait until Exec is called
	handle.cancel() // cancel now that we're blocked in exec

	select {
	case <-handle.wait():
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for tasklet check to exit")
	}

	// The underlying ScriptExecutor (newBlockScriptExec) *cannot* be
	// canceled. Only a wrapper around it obeys the context cancelation.
	if atomic.LoadInt32(&exec.exited) == 1 {
		t.Errorf("expected script executor to still be running after timeout")
	}
	// No tasklets finished, so no callbacks should have gotten a
	// chance to fire
	select {
	case call := <-tm.calls:
		t.Errorf("expected 0 calls of tasklet, got %v", call)
	default:
		break
	}
}

// TestTasklet_Exec_Timeout asserts a tasklet script will be killed
// when the timeout is reached.
func TestTasklet_Exec_Timeout(t *testing.T) {
	ci.Parallel(t)
	exec, cancel := newBlockingScriptExec()
	defer cancel()

	tm := newTaskletMock(exec, testlog.HCLogger(t), time.Hour, time.Second)

	handle := tm.run()
	defer handle.cancel() // just-in-case cleanup
	<-exec.running        // wait until Exec is called

	// We should get a timeout
	select {
	case update := <-tm.calls:
		if update.err != context.DeadlineExceeded {
			t.Errorf("expected context.DeadlineExceeed but received %+v", update)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}

	// The underlying ScriptExecutor (newBlockScriptExec) *cannot* be
	// canceled. Only a wrapper around it obeys the context cancelation.
	if atomic.LoadInt32(&exec.exited) == 1 {
		t.Errorf("expected executor to still be running after timeout")
	}

	// Cancel and watch for exit
	handle.cancel()
	select {
	case <-handle.wait(): // ok!
	case update := <-tm.calls:
		t.Errorf("unexpected extra callback on exit with status=%v", update)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for tasklet to exit")
	}
}

// TestTasklet_Exec_Shutdown asserts a script will be executed once more
// when told to shutdown.
func TestTasklet_Exec_Shutdown(t *testing.T) {
	ci.Parallel(t)

	exec := newSimpleExec(0, nil)
	shutdown := make(chan struct{})
	tm := newTaskletMock(exec, testlog.HCLogger(t), time.Hour, 3*time.Second)
	tm.shutdownCh = shutdown
	handle := tm.run()

	defer handle.cancel() // just-in-case cleanup
	close(shutdown)       // tell script to exit

	select {
	case update := <-tm.calls:
		if update.err != nil {
			t.Errorf("expected clean shutdown but received %q", update.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}

	select {
	case <-handle.wait(): // ok
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}
}

// test helpers

type taskletMock struct {
	tasklet
	calls chan execResult
}

func newTaskletMock(exec interfaces.ScriptExecutor, logger hclog.Logger, interval, timeout time.Duration) *taskletMock {
	tm := &taskletMock{calls: make(chan execResult)}
	tm.exec = exec
	tm.logger = logger
	tm.Interval = interval
	tm.Timeout = timeout
	tm.callback = func(ctx context.Context, params execResult) {
		tm.calls <- params
	}
	return tm
}

// blockingScriptExec implements ScriptExec by running a subcommand that never
// exits.
type blockingScriptExec struct {
	// pctx is canceled *only* for test cleanup. Just like real
	// ScriptExecutors its Exec method cannot be canceled directly -- only
	// with a timeout.
	pctx context.Context

	// running is ticked before blocking to allow synchronizing operations
	running chan struct{}

	// set to 1 with atomics if Exec is called and has exited
	exited int32
}

// newBlockingScriptExec returns a ScriptExecutor that blocks Exec() until the
// caller recvs on the b.running chan. It also returns a CancelFunc for test
// cleanup only. The runtime cannot cancel ScriptExecutors before their timeout
// expires.
func newBlockingScriptExec() (*blockingScriptExec, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	exec := &blockingScriptExec{
		pctx:    ctx,
		running: make(chan struct{}),
	}
	return exec, cancel
}

func (b *blockingScriptExec) Exec(dur time.Duration, _ string, _ []string) ([]byte, int, error) {
	b.running <- struct{}{}
	ctx, cancel := context.WithTimeout(b.pctx, dur)
	defer cancel()
	cmd := exec.CommandContext(ctx, testtask.Path(), "sleep", "9000h")
	testtask.SetCmdEnv(cmd)
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		if !exitErr.Success() {
			code = 1
		}
	}
	atomic.StoreInt32(&b.exited, 1)
	return []byte{}, code, err
}

// sleeperExec sleeps for 100ms but returns successfully to allow testing timeout conditions
type sleeperExec struct{}

func (sleeperExec) Exec(time.Duration, string, []string) ([]byte, int, error) {
	time.Sleep(100 * time.Millisecond)
	return []byte{}, 0, nil
}

// simpleExec is a fake ScriptExecutor that returns whatever is specified.
type simpleExec struct {
	code int
	err  error
}

func (s simpleExec) Exec(time.Duration, string, []string) ([]byte, int, error) {
	return []byte(fmt.Sprintf("code=%d err=%v", s.code, s.err)), s.code, s.err
}

// newSimpleExec creates a new ScriptExecutor that returns the given code and err.
func newSimpleExec(code int, err error) simpleExec {
	return simpleExec{code: code, err: err}
}

// scriptedExec is a fake ScriptExecutor with a predetermined sequence
// of results.
type scriptedExec struct {
	fn func() ([]byte, int, error)
}

// For each call to Exec, scriptedExec returns the next result in its
// sequence of results
func (s scriptedExec) Exec(time.Duration, string, []string) ([]byte, int, error) {
	return s.fn()
}

func newScriptedExec(results []execResult) scriptedExec {
	index := 0
	s := scriptedExec{}
	// we have to close over the index because the interface we're
	// mocking expects a value and not a pointer, which prevents
	// us from updating the index
	fn := func() ([]byte, int, error) {
		result := results[index]
		// prevents us from iterating off the end of the results
		if index+1 < len(results) {
			index = index + 1
		}
		return result.output, result.code, result.err
	}
	s.fn = fn
	return s
}
