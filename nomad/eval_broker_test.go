// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

var (
	defaultSched = []string{
		structs.JobTypeService,
		structs.JobTypeBatch,
	}
)

func testBrokerConfig() *Config {
	config := DefaultConfig()

	// Tune the Nack timeout
	config.EvalNackTimeout = 5 * time.Second

	// Tune the Nack delay
	config.EvalNackInitialReenqueueDelay = 5 * time.Millisecond
	config.EvalNackSubsequentReenqueueDelay = 50 * time.Millisecond
	return config
}

func testBroker(t *testing.T, timeout time.Duration) *EvalBroker {
	config := testBrokerConfig()

	if timeout != 0 {
		config.EvalNackTimeout = timeout
	}

	return testBrokerFromConfig(t, config)
}

func testBrokerFromConfig(t *testing.T, c *Config) *EvalBroker {
	b, err := NewEvalBroker(c.EvalNackTimeout, c.EvalNackInitialReenqueueDelay, c.EvalNackSubsequentReenqueueDelay, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	return b
}

func TestEvalBroker_Enqueue_Dequeue_Nack_Ack(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)

	// Enqueue, but broker is disabled!
	eval := mock.Eval()
	b.Enqueue(eval)

	// Verify nothing was done
	stats := b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}

	if b.Enabled() {
		t.Fatalf("should not be enabled")
	}

	// Enable the broker, and enqueue
	b.SetEnabled(true)
	b.Enqueue(eval)

	// Double enqueue is a no-op
	b.Enqueue(eval)

	if !b.Enabled() {
		t.Fatalf("should be enabled")
	}

	// Verify enqueue is done
	stats = b.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Ready != 1 {
		t.Fatalf("bad: %#v", stats)
	}

	// Dequeue should work
	out, token, err := b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad : %#v", out)
	}

	tokenOut, ok := b.Outstanding(out.ID)
	if !ok {
		t.Fatalf("should be outstanding")
	}
	if tokenOut != token {
		t.Fatalf("Bad: %#v %#v", token, tokenOut)
	}

	// OutstandingReset should verify the token
	err = b.OutstandingReset("nope", "foo")
	if err != ErrNotOutstanding {
		t.Fatalf("err: %v", err)
	}
	err = b.OutstandingReset(out.ID, "foo")
	if err != ErrTokenMismatch {
		t.Fatalf("err: %v", err)
	}
	err = b.OutstandingReset(out.ID, tokenOut)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the stats
	stats = b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Ready != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Unacked != 1 {
		t.Fatalf("bad: %#v", stats)
	}

	// Nack with wrong token should fail
	err = b.Nack(eval.ID, "foobarbaz")
	if err == nil {
		t.Fatalf("should fail to nack")
	}

	// Nack back into the queue
	err = b.Nack(eval.ID, token)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := b.Outstanding(out.ID); ok {
		t.Fatalf("should not be outstanding")
	}

	// Check the stats
	testutil.WaitForResult(func() (bool, error) {
		stats = b.Stats()
		if stats.TotalReady != 1 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.TotalUnacked != 0 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.TotalWaiting != 0 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.ByScheduler[eval.Type].Ready != 1 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.ByScheduler[eval.Type].Unacked != 0 {
			return false, fmt.Errorf("bad: %#v", stats)
		}

		return true, nil
	}, func(e error) {
		t.Fatal(e)
	})

	// Dequeue should work again
	out2, token2, err := b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out2 != eval {
		t.Fatalf("bad : %#v", out2)
	}
	if token2 == token {
		t.Fatalf("should get a new token")
	}

	tokenOut2, ok := b.Outstanding(out.ID)
	if !ok {
		t.Fatalf("should be outstanding")
	}
	if tokenOut2 != token2 {
		t.Fatalf("Bad: %#v %#v", token2, tokenOut2)
	}

	// Ack with wrong token
	err = b.Ack(eval.ID, "zip")
	if err == nil {
		t.Fatalf("should fail to ack")
	}

	// Ack finally
	err = b.Ack(eval.ID, token2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := b.Outstanding(out.ID); ok {
		t.Fatalf("should not be outstanding")
	}

	// Check the stats
	stats = b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Ready != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Unacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
}

func TestEvalBroker_Nack_Delay(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)

	// Enqueue, but broker is disabled!
	b.SetEnabled(true)
	eval := mock.Eval()
	b.Enqueue(eval)

	// Dequeue should work
	out, token, err := b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad : %#v", out)
	}

	// Nack back into the queue
	err = b.Nack(eval.ID, token)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := b.Outstanding(out.ID); ok {
		t.Fatalf("should not be outstanding")
	}

	// Check the stats to ensure that it is waiting
	stats := b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalWaiting != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Ready != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Unacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}

	// Now wait for it to be re-enqueued
	testutil.WaitForResult(func() (bool, error) {
		stats = b.Stats()
		if stats.TotalReady != 1 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.TotalUnacked != 0 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.TotalWaiting != 0 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.ByScheduler[eval.Type].Ready != 1 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.ByScheduler[eval.Type].Unacked != 0 {
			return false, fmt.Errorf("bad: %#v", stats)
		}

		return true, nil
	}, func(e error) {
		t.Fatal(e)
	})

	// Dequeue should work again
	out2, token2, err := b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out2 != eval {
		t.Fatalf("bad : %#v", out2)
	}
	if token2 == token {
		t.Fatalf("should get a new token")
	}

	// Capture the time
	start := time.Now()

	// Nack back into the queue
	err = b.Nack(eval.ID, token2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now wait for it to be re-enqueued
	testutil.WaitForResult(func() (bool, error) {
		stats = b.Stats()
		if stats.TotalReady != 1 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.TotalUnacked != 0 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.TotalWaiting != 0 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.ByScheduler[eval.Type].Ready != 1 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.ByScheduler[eval.Type].Unacked != 0 {
			return false, fmt.Errorf("bad: %#v", stats)
		}

		return true, nil
	}, func(e error) {
		t.Fatal(e)
	})

	delay := time.Now().Sub(start)
	if delay < b.subsequentNackDelay {
		t.Fatalf("bad: delay was %v; want at least %v", delay, b.subsequentNackDelay)
	}

	// Dequeue should work again
	out3, token3, err := b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out3 != eval {
		t.Fatalf("bad : %#v", out3)
	}
	if token3 == token || token3 == token2 {
		t.Fatalf("should get a new token")
	}

	// Ack finally
	err = b.Ack(eval.ID, token3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := b.Outstanding(out.ID); ok {
		t.Fatalf("should not be outstanding")
	}

	// Check the stats
	stats = b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Ready != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Unacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
}

func TestEvalBroker_Serialize_DuplicateJobID(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	ns1 := "namespace-one"
	ns2 := "namespace-two"
	jobID := "example"

	newEval := func(idx uint64, ns string) *structs.Evaluation {
		eval := mock.Eval()
		eval.ID = fmt.Sprintf("eval:%d", idx)
		eval.JobID = jobID
		eval.Namespace = ns
		eval.CreateIndex = idx
		eval.ModifyIndex = idx
		b.Enqueue(eval)
		return eval
	}

	// first job
	eval1 := newEval(1, ns1)
	newEval(2, ns1)
	newEval(3, ns1)
	eval4 := newEval(4, ns1)

	// second job
	eval5 := newEval(5, ns2)
	newEval(6, ns2)
	eval7 := newEval(7, ns2)

	// retreive the stats from the broker, less some stats that aren't
	// interesting for this test and make the test much more verbose
	// to include
	getStats := func() BrokerStats {
		t.Helper()
		stats := b.Stats()
		stats.DelayedEvals = nil
		stats.ByScheduler = nil
		return *stats
	}

	must.Eq(t, BrokerStats{TotalReady: 2, TotalUnacked: 0,
		TotalPending: 5, TotalCancelable: 0}, getStats())

	// Dequeue should get 1st eval
	out, token, err := b.Dequeue(defaultSched, time.Second)
	must.NoError(t, err)
	must.Eq(t, out, eval1, must.Sprint("expected 1st eval"))

	must.Eq(t, BrokerStats{TotalReady: 1, TotalUnacked: 1,
		TotalPending: 5, TotalCancelable: 0}, getStats())

	// Current wait index should be 4 but Ack to exercise behavior
	// when worker's Eval.getWaitIndex gets a stale index
	err = b.Ack(eval1.ID, token)
	must.NoError(t, err)

	must.Eq(t, BrokerStats{TotalReady: 2, TotalUnacked: 0,
		TotalPending: 2, TotalCancelable: 2}, getStats())

	// eval4 and eval5 are ready
	// eval6 and eval7 are pending
	// Dequeue should get 4th eval
	out, token, err = b.Dequeue(defaultSched, time.Second)
	must.NoError(t, err)
	must.Eq(t, out, eval4, must.Sprint("expected 4th eval"))

	must.Eq(t, BrokerStats{TotalReady: 1, TotalUnacked: 1,
		TotalPending: 2, TotalCancelable: 2}, getStats())

	// Ack should clear the rest of namespace-one pending but leave
	// namespace-two untouched
	err = b.Ack(eval4.ID, token)
	must.NoError(t, err)

	must.Eq(t, BrokerStats{TotalReady: 1, TotalUnacked: 0,
		TotalPending: 2, TotalCancelable: 2}, getStats())

	// Dequeue should get 5th eval
	out, token, err = b.Dequeue(defaultSched, time.Second)
	must.NoError(t, err)
	must.Eq(t, out, eval5, must.Sprint("expected 5th eval"))

	must.Eq(t, BrokerStats{TotalReady: 0, TotalUnacked: 1,
		TotalPending: 2, TotalCancelable: 2}, getStats())

	// Ack should clear remaining namespace-two pending evals
	err = b.Ack(eval5.ID, token)
	must.NoError(t, err)

	must.Eq(t, BrokerStats{TotalReady: 1, TotalUnacked: 0,
		TotalPending: 0, TotalCancelable: 3}, getStats())

	// Dequeue should get 7th eval because that's all that's left
	out, token, err = b.Dequeue(defaultSched, time.Second)
	must.NoError(t, err)
	must.Eq(t, out, eval7, must.Sprint("expected 7th eval"))

	must.Eq(t, BrokerStats{TotalReady: 0, TotalUnacked: 1,
		TotalPending: 0, TotalCancelable: 3}, getStats())

	// Last ack should leave the broker empty except for cancels
	err = b.Ack(eval7.ID, token)
	must.NoError(t, err)

	must.Eq(t, BrokerStats{TotalReady: 0, TotalUnacked: 0,
		TotalPending: 0, TotalCancelable: 3}, getStats())
}

func TestEvalBroker_Enqueue_Disable(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)

	// Enqueue
	eval := mock.Eval()
	b.SetEnabled(true)
	b.Enqueue(eval)

	// Flush via SetEnabled
	b.SetEnabled(false)

	// Check the stats
	stats := b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if _, ok := stats.ByScheduler[eval.Type]; ok {
		t.Fatalf("bad: %#v", stats)
	}
}

func TestEvalBroker_Enqueue_Disable_Delay(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	baseEval := mock.Eval()
	b.SetEnabled(true)

	{
		// Enqueue
		b.Enqueue(baseEval.Copy())

		delayedEval := baseEval.Copy()
		delayedEval.Wait = 30
		b.Enqueue(delayedEval)

		waitEval := baseEval.Copy()
		waitEval.WaitUntil = time.Now().Add(30 * time.Second)
		b.Enqueue(waitEval)
	}

	// Flush via SetEnabled
	b.SetEnabled(false)

	{
		// Check the stats
		stats := b.Stats()
		require.Equal(t, 0, stats.TotalReady, "Expected ready to be flushed")
		require.Equal(t, 0, stats.TotalWaiting, "Expected waiting to be flushed")
		require.Equal(t, 0, stats.TotalPending, "Expected pending to be flushed")
		require.Equal(t, 0, stats.TotalUnacked, "Expected unacked to be flushed")
		_, ok := stats.ByScheduler[baseEval.Type]
		require.False(t, ok, "Expected scheduler to have no stats")
	}

	{
		// Enqueue again now we're disabled
		b.Enqueue(baseEval.Copy())

		delayedEval := baseEval.Copy()
		delayedEval.Wait = 30 * time.Second
		b.Enqueue(delayedEval)

		waitEval := baseEval.Copy()
		waitEval.WaitUntil = time.Now().Add(30 * time.Second)
		b.Enqueue(waitEval)
	}

	{
		// Check the stats again
		stats := b.Stats()
		require.Equal(t, 0, stats.TotalReady, "Expected ready to be flushed")
		require.Equal(t, 0, stats.TotalWaiting, "Expected waiting to be flushed")
		require.Equal(t, 0, stats.TotalPending, "Expected pending to be flushed")
		require.Equal(t, 0, stats.TotalUnacked, "Expected unacked to be flushed")
		_, ok := stats.ByScheduler[baseEval.Type]
		require.False(t, ok, "Expected scheduler to have no stats")
	}
}

func TestEvalBroker_Dequeue_Timeout(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	start := time.Now()
	out, _, err := b.Dequeue(defaultSched, 5*time.Millisecond)
	end := time.Now()

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected: %#v", out)
	}

	if diff := end.Sub(start); diff < 5*time.Millisecond {
		t.Fatalf("bad: %#v", diff)
	}
}

func TestEvalBroker_Dequeue_Empty_Timeout(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		out, _, err := b.Dequeue(defaultSched, 0)
		if err != nil {
			errCh <- err
			return
		}
		if out == nil {
			errCh <- errors.New("expected a non-nil value")
			return
		}
	}()

	// Sleep for a little bit
	select {
	case <-time.After(5 * time.Millisecond):
	case err := <-errCh:
		if err != nil {
			t.Fatalf("error from dequeue goroutine: %s", err)
		}
		t.Fatalf("Dequeue(0) should block, not finish")
	}

	// Enqueue to unblock the dequeue.
	eval := mock.Eval()
	b.Enqueue(eval)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("error from dequeue goroutine: %s", err)
		}
	case <-time.After(5 * time.Millisecond):
		t.Fatal("timeout: Dequeue(0) should return after enqueue")
	}
}

// Ensure higher priority dequeued first
func TestEvalBroker_Dequeue_Priority(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	eval1 := mock.Eval()
	eval1.Priority = 10
	b.Enqueue(eval1)

	eval2 := mock.Eval()
	eval2.Priority = 30
	b.Enqueue(eval2)

	eval3 := mock.Eval()
	eval3.Priority = 20
	b.Enqueue(eval3)

	out1, _, _ := b.Dequeue(defaultSched, time.Second)
	if out1 != eval2 {
		t.Fatalf("bad: %#v", out1)
	}

	out2, _, _ := b.Dequeue(defaultSched, time.Second)
	if out2 != eval3 {
		t.Fatalf("bad: %#v", out2)
	}

	out3, _, _ := b.Dequeue(defaultSched, time.Second)
	if out3 != eval1 {
		t.Fatalf("bad: %#v", out3)
	}
}

// Ensure FIFO at fixed priority
func TestEvalBroker_Dequeue_FIFO(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)
	NUM := 100

	for i := NUM; i > 0; i-- {
		eval1 := mock.Eval()
		eval1.CreateIndex = uint64(i)
		eval1.ModifyIndex = uint64(i)
		b.Enqueue(eval1)
	}

	for i := 1; i < NUM; i++ {
		out1, _, _ := b.Dequeue(defaultSched, time.Second)
		must.Eq(t, uint64(i), out1.CreateIndex,
			must.Sprintf("eval was not FIFO by CreateIndex"),
		)
	}
}

// Ensure fairness between schedulers
func TestEvalBroker_Dequeue_Fairness(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)
	NUM := 1000

	for i := 0; i < NUM; i++ {
		eval1 := mock.Eval()
		if i < (NUM / 2) {
			eval1.Type = structs.JobTypeService
		} else {
			eval1.Type = structs.JobTypeBatch
		}
		b.Enqueue(eval1)
	}

	counter := 0
	for i := 0; i < NUM; i++ {
		out1, _, _ := b.Dequeue(defaultSched, time.Second)

		switch out1.Type {
		case structs.JobTypeService:
			if counter < 0 {
				counter = 0
			}
			counter += 1
		case structs.JobTypeBatch:
			if counter > 0 {
				counter = 0
			}
			counter -= 1
		}

		// This will fail randomly at times. It is very hard to
		// test deterministically that its acting randomly.
		if counter >= 250 || counter <= -250 {
			t.Fatalf("unlikely sequence: %d", counter)
		}
	}
}

// Ensure we get unblocked
func TestEvalBroker_Dequeue_Blocked(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	// Start with a blocked dequeue
	outCh := make(chan *structs.Evaluation)
	errCh := make(chan error)
	go func() {
		defer close(errCh)
		defer close(outCh)
		start := time.Now()
		out, _, err := b.Dequeue(defaultSched, time.Second)
		if err != nil {
			errCh <- err
			return
		}
		end := time.Now()
		if d := end.Sub(start); d < 5*time.Millisecond {
			errCh <- fmt.Errorf("test broker dequeue duration too fast: %v", d)
			return
		}
		outCh <- out
	}()

	// Wait for a bit, or t.Fatal if an error has already happened in
	// the goroutine
	select {
	case <-time.After(5 * time.Millisecond):
		// no errors yet, soldier on
	case err := <-errCh:
		if err != nil {
			t.Fatalf("error from anonymous goroutine before enqueue: %v", err)
		}
	}

	// Enqueue
	eval := mock.Eval()
	b.Enqueue(eval)

	// Ensure dequeue
	select {
	case out := <-outCh:
		if out != eval {
			prettyExp, _ := json.MarshalIndent(eval, "", "\t")
			prettyGot, _ := json.MarshalIndent(out, "", "\t")
			t.Fatalf("dequeue result expected:\n%s\ngot:\n%s",
				string(prettyExp), string(prettyGot))
		}
	case err := <-errCh:
		t.Fatalf("error from anonymous goroutine after enqueue: %v", err)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for dequeue result")
	}
}

// Ensure we nack in a timely manner
func TestEvalBroker_Nack_Timeout(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 5*time.Millisecond)
	b.SetEnabled(true)

	// Enqueue
	eval := mock.Eval()
	b.Enqueue(eval)

	// Dequeue
	out, _, err := b.Dequeue(defaultSched, time.Second)
	start := time.Now()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad: %v", out)
	}

	// Dequeue, should block on Nack timer
	out, _, err = b.Dequeue(defaultSched, time.Second)
	end := time.Now()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad: %v", out)
	}

	// Check the nack timer
	if diff := end.Sub(start); diff < 5*time.Millisecond {
		t.Fatalf("bad: %#v", diff)
	}
}

// Ensure we nack in a timely manner
func TestEvalBroker_Nack_TimeoutReset(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 50*time.Millisecond)
	b.SetEnabled(true)

	// Enqueue
	eval := mock.Eval()
	b.Enqueue(eval)

	// Dequeue
	out, token, err := b.Dequeue(defaultSched, time.Second)
	start := time.Now()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad: %v", out)
	}

	// Reset in 20 milliseconds
	time.Sleep(20 * time.Millisecond)
	if err := b.OutstandingReset(out.ID, token); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Dequeue, should block on Nack timer
	out, _, err = b.Dequeue(defaultSched, time.Second)
	end := time.Now()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad: %v", out)
	}

	// Check the nack timer
	if diff := end.Sub(start); diff < 75*time.Millisecond {
		t.Fatalf("bad: %#v", diff)
	}
}

func TestEvalBroker_PauseResumeNackTimeout(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 50*time.Millisecond)
	b.SetEnabled(true)

	// Enqueue
	eval := mock.Eval()
	b.Enqueue(eval)

	// Dequeue
	out, token, err := b.Dequeue(defaultSched, time.Second)
	start := time.Now()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad: %v", out)
	}

	// Pause in 20 milliseconds
	time.Sleep(20 * time.Millisecond)
	if err := b.PauseNackTimeout(out.ID, token); err != nil {
		t.Fatalf("pause nack timeout error: %v", err)
	}

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		time.Sleep(20 * time.Millisecond)
		if err := b.ResumeNackTimeout(out.ID, token); err != nil {
			errCh <- err
			return
		}
	}()

	// Dequeue, should block until the timer is resumed
	out, _, err = b.Dequeue(defaultSched, time.Second)
	end := time.Now()
	if err != nil {
		t.Fatalf("dequeue error: %v", err)
	}
	if out != eval {
		prettyExp, _ := json.MarshalIndent(eval, "", "\t")
		prettyGot, _ := json.MarshalIndent(out, "", "\t")
		t.Fatalf("dequeue result expected:\n%s\ngot:\n%s",
			string(prettyExp), string(prettyGot))
	}

	// Check the nack timer
	if diff := end.Sub(start); diff < 95*time.Millisecond {
		t.Fatalf("deqeue happened too fast: %#v", diff)
	}

	// check the result of ResumeNackTimeout
	err = <-errCh
	if err != nil {
		t.Fatalf("resume nack timeout error:%s", err)
	}
}

func TestEvalBroker_DeliveryLimit(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	eval := mock.Eval()
	b.Enqueue(eval)

	for i := 0; i < 3; i++ {
		// Dequeue should work
		out, token, err := b.Dequeue(defaultSched, time.Second)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if out != eval {
			t.Fatalf("bad : %#v", out)
		}

		// Nack with wrong token should fail
		err = b.Nack(eval.ID, token)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Check the stats
	stats := b.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[failedQueue].Ready != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[failedQueue].Unacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}

	// Dequeue from failed queue
	out, token, err := b.Dequeue([]string{failedQueue}, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad : %#v", out)
	}

	// Check the stats
	stats = b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[failedQueue].Ready != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[failedQueue].Unacked != 1 {
		t.Fatalf("bad: %#v", stats)
	}

	// Ack finally
	err = b.Ack(out.ID, token)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := b.Outstanding(out.ID); ok {
		t.Fatalf("should not be outstanding")
	}

	// Check the stats
	stats = b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[failedQueue].Ready != 0 {
		t.Fatalf("bad: %#v", stats.ByScheduler[failedQueue])
	}
	if stats.ByScheduler[failedQueue].Unacked != 0 {
		t.Fatalf("bad: %#v", stats.ByScheduler[failedQueue])
	}
}

func TestEvalBroker_AckAtDeliveryLimit(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	eval := mock.Eval()
	b.Enqueue(eval)

	for i := 0; i < 3; i++ {
		// Dequeue should work
		out, token, err := b.Dequeue(defaultSched, time.Second)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if out != eval {
			t.Fatalf("bad : %#v", out)
		}

		if i == 2 {
			b.Ack(eval.ID, token)
		} else {
			// Nack with wrong token should fail
			err = b.Nack(eval.ID, token)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
		}
	}

	// Check the stats
	stats := b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if _, ok := stats.ByScheduler[failedQueue]; ok {
		t.Fatalf("bad: %#v", stats)
	}
}

// TestEvalBroker_Wait asserts delayed evaluations cannot be dequeued until
// their wait duration has elapsed.
func TestEvalBroker_Wait(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	// Create an eval that should wait
	eval := mock.Eval()
	eval.Wait = 100 * time.Millisecond
	b.Enqueue(eval)

	// Verify waiting
	stats := b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalWaiting != 1 {
		t.Fatalf("bad: %#v", stats)
	}

	// Dequeue should not return the eval until wait has elapsed
	out, token, err := b.Dequeue(defaultSched, 1)
	require.Nil(t, out)
	require.Empty(t, token)
	require.NoError(t, err)

	// Let the wait elapse
	time.Sleep(200 * time.Millisecond)

	// Verify ready
	stats = b.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalWaiting != 0 {
		t.Fatalf("bad: %#v", stats)
	}

	// Dequeue should work
	out, _, err = b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad : %#v", out)
	}
}

// Ensure that delayed evaluations work as expected
func TestEvalBroker_WaitUntil(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	now := time.Now()
	// Create a few of evals with WaitUntil set
	eval1 := mock.Eval()
	eval1.WaitUntil = now.Add(1 * time.Second)
	eval1.CreateIndex = 1
	b.Enqueue(eval1)

	eval2 := mock.Eval()
	eval2.WaitUntil = now.Add(100 * time.Millisecond)
	// set CreateIndex to use as a tie breaker when eval2
	// and eval3 are both in the pending evals heap
	eval2.CreateIndex = 2
	b.Enqueue(eval2)

	eval3 := mock.Eval()
	eval3.WaitUntil = now.Add(20 * time.Millisecond)
	eval3.CreateIndex = 1
	b.Enqueue(eval3)
	require.Equal(3, b.stats.TotalWaiting)
	// sleep enough for two evals to be ready
	time.Sleep(200 * time.Millisecond)

	// first dequeue should return eval3
	out, _, err := b.Dequeue(defaultSched, time.Second)
	require.Nil(err)
	require.Equal(eval3, out)

	// second dequeue should return eval2
	out, _, err = b.Dequeue(defaultSched, time.Second)
	require.Nil(err)
	require.Equal(eval2, out)

	// third dequeue should return eval1
	out, _, err = b.Dequeue(defaultSched, 2*time.Second)
	require.Nil(err)
	require.Equal(eval1, out)
	require.Equal(0, b.stats.TotalWaiting)
}

// Ensure that priority is taken into account when enqueueing many evaluations.
func TestEvalBroker_EnqueueAll_Dequeue_Fair(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	// Start with a blocked dequeue
	outCh := make(chan *structs.Evaluation)
	errCh := make(chan error)
	go func() {
		defer close(errCh)
		defer close(outCh)
		start := time.Now()
		out, _, err := b.Dequeue(defaultSched, time.Second)
		if err != nil {
			errCh <- err
			return
		}
		end := time.Now()
		if d := end.Sub(start); d < 5*time.Millisecond {
			errCh <- fmt.Errorf("test broker dequeue duration too fast: %v", d)
			return
		}
		outCh <- out
	}()

	// Wait for a bit, or t.Fatal if an error has already happened in
	// the goroutine
	select {
	case <-time.After(5 * time.Millisecond):
		// no errors yet, soldier on
	case err := <-errCh:
		if err != nil {
			t.Fatalf("error from anonymous goroutine before enqueue: %v", err)
		}
	}

	// Enqueue
	evals := make(map[*structs.Evaluation]string, 8)
	expectedPriority := 90
	for i := 10; i <= expectedPriority; i += 10 {
		eval := mock.Eval()
		eval.Priority = i
		evals[eval] = ""

	}
	b.EnqueueAll(evals)

	// Ensure dequeue
	select {
	case out := <-outCh:
		if out.Priority != expectedPriority {
			pretty, _ := json.MarshalIndent(out, "", "\t")
			t.Logf("bad priority on *structs.Evaluation: %s", string(pretty))
			t.Fatalf("priority wanted:%d, priority got:%d", expectedPriority, out.Priority)
		}
	case err := <-errCh:
		t.Fatalf("error from anonymous goroutine after enqueue: %v", err)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for dequeue result")
	}
}

func TestEvalBroker_EnqueueAll_Requeue_Ack(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	// Create the evaluation, enqueue and dequeue
	eval := mock.Eval()
	b.Enqueue(eval)

	out, token, err := b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad : %#v", out)
	}

	// Requeue the same evaluation.
	b.EnqueueAll(map[*structs.Evaluation]string{eval: token})

	// The stats should show one unacked
	stats := b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 1 {
		t.Fatalf("bad: %#v", stats)
	}

	// Ack the evaluation.
	if err := b.Ack(eval.ID, token); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check stats again as this should cause the re-enqueued one to transition
	// into the ready state
	stats = b.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}

	// Another dequeue should be successful
	out2, token2, err := b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out2 != eval {
		t.Fatalf("bad : %#v", out)
	}
	if token == token2 {
		t.Fatalf("bad : %s and %s", token, token2)
	}
}

func TestEvalBroker_EnqueueAll_Requeue_Nack(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	// Create the evaluation, enqueue and dequeue
	eval := mock.Eval()
	b.Enqueue(eval)

	out, token, err := b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad : %#v", out)
	}

	// Requeue the same evaluation.
	b.EnqueueAll(map[*structs.Evaluation]string{eval: token})

	// The stats should show one unacked
	stats := b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 1 {
		t.Fatalf("bad: %#v", stats)
	}

	// Nack the evaluation.
	if err := b.Nack(eval.ID, token); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check stats again as this should cause the re-enqueued one to be dropped
	testutil.WaitForResult(func() (bool, error) {
		stats = b.Stats()
		if stats.TotalReady != 1 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if stats.TotalUnacked != 0 {
			return false, fmt.Errorf("bad: %#v", stats)
		}
		if len(b.requeue) != 0 {
			return false, fmt.Errorf("bad: %#v", b.requeue)
		}

		return true, nil
	}, func(e error) {
		t.Fatal(e)
	})
}

func TestEvalBroker_NamespacedJobs(t *testing.T) {
	ci.Parallel(t)
	b := testBroker(t, 0)
	b.SetEnabled(true)

	// Create evals with the same jobid and different namespace
	jobId := "test-jobID"

	eval1 := mock.Eval()
	eval1.JobID = jobId
	eval1.Namespace = "n1"
	b.Enqueue(eval1)

	// This eval should not block
	eval2 := mock.Eval()
	eval2.JobID = jobId
	eval2.Namespace = "default"
	b.Enqueue(eval2)

	// This eval should block
	eval3 := mock.Eval()
	eval3.JobID = jobId
	eval3.Namespace = "default"
	b.Enqueue(eval3)

	require := require.New(t)
	out1, _, err := b.Dequeue(defaultSched, 5*time.Millisecond)
	require.Nil(err)
	require.Equal(eval1.ID, out1.ID)

	out2, _, err := b.Dequeue(defaultSched, 5*time.Millisecond)
	require.Nil(err)
	require.Equal(eval2.ID, out2.ID)

	out3, _, err := b.Dequeue(defaultSched, 5*time.Millisecond)
	require.Nil(err)
	require.Nil(out3)

	require.Equal(1, len(b.pending))

}

func TestEvalBroker_ReadyEvals_Ordering(t *testing.T) {

	ready := ReadyEvaluations{}

	newEval := func(jobID, evalID string, priority int, index uint64) *structs.Evaluation {
		eval := mock.Eval()
		eval.JobID = jobID
		eval.ID = evalID
		eval.Priority = priority
		eval.CreateIndex = uint64(index)
		return eval
	}

	// note: we're intentionally pushing these out-of-order to assert we're
	// getting them back out in the intended order and not just as inserted
	heap.Push(&ready, newEval("example1", "eval01", 50, 1))
	heap.Push(&ready, newEval("example3", "eval03", 70, 3))
	heap.Push(&ready, newEval("example2", "eval02", 50, 2))

	next := heap.Pop(&ready).(*structs.Evaluation)
	test.Eq(t, "eval03", next.ID,
		test.Sprint("expected highest Priority to be next ready"))

	next = heap.Pop(&ready).(*structs.Evaluation)
	test.Eq(t, "eval01", next.ID,
		test.Sprint("expected oldest CreateIndex to be next ready"))

	heap.Push(&ready, newEval("example4", "eval04", 50, 4))

	next = heap.Pop(&ready).(*structs.Evaluation)
	test.Eq(t, "eval02", next.ID,
		test.Sprint("expected oldest CreateIndex to be next ready"))

}

func TestEvalBroker_PendingEval_Ordering(t *testing.T) {
	pending := PendingEvaluations{}

	newEval := func(evalID string, priority int, index uint64) *structs.Evaluation {
		eval := mock.Eval()
		eval.ID = evalID
		eval.Priority = priority
		eval.ModifyIndex = uint64(index)
		return eval
	}

	// note: we're intentionally pushing these out-of-order to assert we're
	// getting them back out in the intended order and not just as inserted
	heap.Push(&pending, newEval("eval03", 50, 3))
	heap.Push(&pending, newEval("eval02", 100, 2))
	heap.Push(&pending, newEval("eval01", 50, 1))

	next := heap.Pop(&pending).(*structs.Evaluation)
	test.Eq(t, "eval02", next.ID,
		test.Sprint("expected eval with highest priority to be next"))

	next = heap.Pop(&pending).(*structs.Evaluation)
	test.Eq(t, "eval03", next.ID,
		test.Sprint("expected eval with highest modify index to be next"))

	heap.Push(&pending, newEval("eval04", 30, 4))
	next = heap.Pop(&pending).(*structs.Evaluation)
	test.Eq(t, "eval01", next.ID,
		test.Sprint("expected eval with highest priority to be nexct"))

}

func TestEvalBroker_PendingEvals_MarkForCancel(t *testing.T) {
	ci.Parallel(t)

	pending := PendingEvaluations{}

	// note: we're intentionally pushing these out-of-order to assert we're
	// getting them back out in the intended order and not just as inserted
	for i := 100; i > 0; i -= 10 {
		eval := mock.Eval()
		eval.JobID = "example"
		eval.CreateIndex = uint64(i)
		eval.ModifyIndex = uint64(i)
		heap.Push(&pending, eval)
	}

	canceled := pending.MarkForCancel()
	must.Eq(t, 9, len(canceled))
	must.Eq(t, 1, pending.Len())

	raw := heap.Pop(&pending)
	must.NotNil(t, raw)
	eval := raw.(*structs.Evaluation)
	must.Eq(t, 100, eval.ModifyIndex)
}

func TestEvalBroker_Cancelable(t *testing.T) {
	ci.Parallel(t)

	b := testBroker(t, time.Minute)

	evals := []*structs.Evaluation{}
	for i := 0; i < 20; i++ {
		eval := mock.Eval()
		evals = append(evals, eval)
	}
	b.cancelable = evals
	b.stats.TotalCancelable = len(b.cancelable)

	must.Len(t, 20, b.cancelable)
	cancelable := b.Cancelable(10)
	must.Len(t, 10, cancelable)
	must.Len(t, 10, b.cancelable)
	must.Eq(t, 10, b.stats.TotalCancelable)

	cancelable = b.Cancelable(20)
	must.Len(t, 10, cancelable)
	must.Len(t, 0, b.cancelable)
	must.Eq(t, 0, b.stats.TotalCancelable)
}

// TestEvalBroker_IntegrationTest exercises the eval broker with realistic
// workflows
func TestEvalBroker_IntegrationTest(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0                             // Prevent dequeue
		c.EvalReapCancelableInterval = time.Minute * 10 // Prevent sweep-up
	})

	defer cleanupS1()
	testutil.WaitForLeader(t, srv.RPC)

	codec := rpcClient(t, srv)
	store := srv.fsm.State()

	// create a system job, a node for it to run on, and a set of node up/down
	// events that will result in evaluations queued.

	job := mock.SystemJob()
	jobReq := &structs.JobRegisterRequest{
		Job:          job,
		EvalPriority: 50,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var jobResp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", jobReq, &jobResp)
	must.NoError(t, err)

	node := mock.Node()
	nodeReq := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	err = msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReq, &nodeResp)
	must.NoError(t, err)

	for i := 0; i < 10; i++ {
		status := structs.NodeStatusDown
		if i%2 == 0 {
			status = structs.NodeStatusReady
		}
		statusReq := &structs.NodeUpdateStatusRequest{
			NodeID:       node.ID,
			Status:       status,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var statusResp structs.NodeUpdateResponse
		err = msgpackrpc.CallWithCodec(codec, "Node.UpdateStatus", statusReq, &statusResp)
		must.NoError(t, err)
	}

	// ensure we have the expected number of evaluations and eval broker state

	// retreive the stats from the broker, less some uninteresting ones
	getStats := func() BrokerStats {
		t.Helper()
		stats := srv.evalBroker.Stats()
		stats.DelayedEvals = nil
		stats.ByScheduler = nil
		return *stats
	}

	getEvalStatuses := func() map[string]int {
		t.Helper()
		statuses := map[string]int{}
		iter, err := store.Evals(nil, state.SortDefault)
		must.NoError(t, err)
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			eval := raw.(*structs.Evaluation)
			statuses[eval.Status] += 1
			if eval.Status == structs.EvalStatusCancelled {
				must.Eq(t, "canceled after more recent eval was processed", eval.StatusDescription)
			}
		}
		return statuses
	}

	must.Eq(t, map[string]int{structs.EvalStatusPending: 11}, getEvalStatuses())
	must.Eq(t, BrokerStats{TotalReady: 1, TotalUnacked: 0,
		TotalPending: 10, TotalCancelable: 0}, getStats())

	// start schedulers: all the evals are for a single job so there should only
	// be one eval processesed at a time no matter how many schedulers we run

	config := DefaultConfig()
	config.NumSchedulers = 4
	must.NoError(t, srv.Reload(config))

	// assert that all but 2 evals were canceled and that the eval broker state
	// has been cleared

	var got map[string]int

	must.Wait(t, wait.InitialSuccess(
		wait.Timeout(5*time.Second),
		wait.Gap(100*time.Millisecond),
		wait.BoolFunc(func() bool {
			got = getEvalStatuses()
			return got[structs.EvalStatusComplete] == 2 &&
				got[structs.EvalStatusCancelled] == 9
		}),
	),
		must.Func(func() string {
			return fmt.Sprintf("expected map[complete:2 canceled:9] within timeout, got: %v with broker status=%#v", got, getStats())
		}),
	)

	must.Eq(t, BrokerStats{TotalReady: 0, TotalUnacked: 0,
		TotalPending: 0, TotalCancelable: 0}, getStats())
}
