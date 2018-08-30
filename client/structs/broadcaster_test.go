package structs

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/stretchr/testify/require"
)

// TestAllocBroadcaster_SendRecv asserts the latest sends to a broadcaster are
// received by listeners.
func TestAllocBroadcaster_SendRecv(t *testing.T) {
	t.Parallel()

	b := NewAllocBroadcaster()
	defer b.Close()

	// Create a listener and assert it blocks until an update
	l := b.Listen()
	defer l.Close()
	select {
	case <-l.Ch:
		t.Fatalf("unexpected initial alloc")
	case <-time.After(10 * time.Millisecond):
		// Ok! Ch is empty until a Send
	}

	// Send an update
	alloc := mock.Alloc()
	alloc.AllocModifyIndex = 10
	require.NoError(t, b.Send(alloc.Copy()))
	recvd := <-l.Ch
	require.Equal(t, alloc.AllocModifyIndex, recvd.AllocModifyIndex)

	// Send two now copies and assert only the last was received
	alloc.AllocModifyIndex = 30
	require.NoError(t, b.Send(alloc.Copy()))
	alloc.AllocModifyIndex = 40
	require.NoError(t, b.Send(alloc.Copy()))

	recvd = <-l.Ch
	require.Equal(t, alloc.AllocModifyIndex, recvd.AllocModifyIndex)
}

// TestAllocBroadcaster_RecvBlocks asserts listeners are blocked until a send occurs.
func TestAllocBroadcaster_RecvBlocks(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	b := NewAllocBroadcaster()
	defer b.Close()

	l1 := b.Listen()
	defer l1.Close()

	l2 := b.Listen()
	defer l2.Close()

	done := make(chan int, 2)

	// Subsequent listens should block until a subsequent send
	go func() {
		<-l1.Ch
		done <- 1
	}()

	go func() {
		<-l2.Ch
		done <- 1
	}()

	select {
	case <-done:
		t.Fatalf("unexpected receive by a listener")
	case <-time.After(10 * time.Millisecond):
	}

	// Do a Send and expect both listeners to receive it
	b.Send(alloc)
	<-done
	<-done
}

// TestAllocBroadcaster_Concurrency asserts that the broadcaster behaves
// correctly with concurrent listeners being added and closed.
func TestAllocBroadcaster_Concurrency(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	b := NewAllocBroadcaster()
	defer b.Close()

	errs := make(chan error, 10)
	listeners := make([]*AllocListener, 10)
	for i := 0; i < len(listeners); i++ {
		l := b.Listen()
		defer l.Close()

		listeners[i] = l
		go func(index uint64, listener *AllocListener) {
			defer listener.Close()
			for {
				a, ok := <-listener.Ch
				if !ok {
					return
				}

				if a.AllocModifyIndex < index {
					errs <- fmt.Errorf("index=%d < %d", a.AllocModifyIndex, index)
					return
				}
				index = a.AllocModifyIndex
			}
		}(alloc.AllocModifyIndex, l)
	}

	for i := 0; i < 100; i++ {
		alloc.AllocModifyIndex++
		require.NoError(t, b.Send(alloc.Copy()))
	}

	if len(errs) > 0 {
		t.Fatalf("%d listener errors. First error:\n%v", len(errs), <-errs)
	}

	// Closing a couple shouldn't cause errors
	listeners[0].Close()
	listeners[1].Close()

	for i := 0; i < 100; i++ {
		alloc.AllocModifyIndex++
		require.NoError(t, b.Send(alloc.Copy()))
	}

	if len(errs) > 0 {
		t.Fatalf("%d listener errors. First error:\n%v", len(errs), <-errs)
	}

	// Closing the broadcaster *should* error
	b.Close()
	require.Equal(t, ErrAllocBroadcasterClosed, b.Send(alloc))

	// All Listeners should be closed
	for _, l := range listeners {
		select {
		case _, ok := <-l.Ch:
			if ok {
				// This check can beat the goroutine above to
				// recv'ing the final update. Listener must be
				// closed on next recv.
				if _, ok := <-l.Ch; ok {
					t.Fatalf("expected listener to be closed")
				}
			}
		default:
			t.Fatalf("expected listener to be closed; not blocking")
		}
	}
}
