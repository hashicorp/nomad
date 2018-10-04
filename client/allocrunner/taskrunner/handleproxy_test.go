package taskrunner

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/driver/structs"
	"github.com/stretchr/testify/require"
)

// TestHandleResult_Wait_Result asserts multiple waiters on a handleResult all
// receive the wait result.
func TestHandleResult_Wait_Result(t *testing.T) {
	t.Parallel()

	waitCh := make(chan *structs.WaitResult)
	h := newHandleResult(waitCh)

	outCh1 := make(chan *structs.WaitResult)
	outCh2 := make(chan *structs.WaitResult)

	// Create two recievers
	go func() {
		outCh1 <- h.Wait(context.Background())
	}()
	go func() {
		outCh2 <- h.Wait(context.Background())
	}()

	// Send a single result
	go func() {
		waitCh <- &structs.WaitResult{ExitCode: 1}
	}()

	// Assert both receivers got the result
	assert := func(outCh chan *structs.WaitResult) {
		select {
		case result := <-outCh:
			require.NotNil(t, result)
			require.Equal(t, 1, result.ExitCode)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for result")
		}
	}

	assert(outCh1)
	assert(outCh2)
}

// TestHandleResult_Wait_Cancel asserts that canceling the context unblocks the
// waiter.
func TestHandleResult_Wait_Cancel(t *testing.T) {
	t.Parallel()

	waitCh := make(chan *structs.WaitResult)
	h := newHandleResult(waitCh)

	ctx, cancel := context.WithCancel(context.Background())
	outCh := make(chan *structs.WaitResult)

	go func() {
		outCh <- h.Wait(ctx)
	}()

	// Cancelling the context should unblock the Wait
	cancel()

	// Assert the result is nil
	select {
	case result := <-outCh:
		require.Nil(t, result)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for result")
	}
}
