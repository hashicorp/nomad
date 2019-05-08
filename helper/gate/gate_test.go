package gate

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGate_NewClosed(t *testing.T) {
	t.Parallel()

	g := NewClosed()

	assertClosed := func() {
		require.True(t, g.IsClosed())
		select {
		case <-g.Wait():
			require.Fail(t, "expected gate to be closed")
		default:
			// Ok!
		}
	}

	assertClosed()
	g.Close()
	assertClosed()

	// Close should be safe to call multiple times
	g.Close()
	assertClosed()

	g.Open()
	require.False(t, g.IsClosed())
	select {
	case <-g.Wait():
		// Ok!
	default:
		require.Fail(t, "expected gate to be open")
	}
}

func TestGate_NewOpen(t *testing.T) {
	t.Parallel()

	g := NewOpen()

	assertOpen := func() {
		require.False(t, g.IsClosed())
		select {
		case <-g.Wait():
			// Ok!
		default:
			require.Fail(t, "expected gate to be open")
		}
	}

	assertOpen()
	g.Open()
	assertOpen()

	// Open should be safe to call multiple times
	g.Open()
	assertOpen()

	g.Close()
	select {
	case <-g.Wait():
		require.Fail(t, "expected gate to be closed")
	default:
		// Ok!
	}
}

// TestGate_Concurrency is meant to be run with the race detector enabled to
// find any races.
func TestGate_Concurrency(t *testing.T) {
	t.Parallel()

	g := NewOpen()
	wg := sync.WaitGroup{}

	// Start closer
	wg.Add(1)
	go func() {
		defer wg.Done()
		dice := rand.New(rand.NewSource(time.Now().UnixNano()))
		for i := 0; i < 1000; i++ {
			g.Close()
			time.Sleep(time.Duration(dice.Int63n(100)))
		}
	}()

	// Start opener
	wg.Add(1)
	go func() {
		defer wg.Done()
		dice := rand.New(rand.NewSource(time.Now().UnixNano()))
		for i := 0; i < 1000; i++ {
			g.Open()
			time.Sleep(time.Duration(dice.Int63n(100)))
		}
	}()

	// Perform reads concurrently with writes
	wgCh := make(chan struct{})
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		for {
			select {
			case <-time.After(time.Millisecond):
			case <-wgCh:
				return
			}
			g.IsClosed()
			g.Wait()
		}
	}()

	wg.Wait()
	close(wgCh)
	<-doneCh
}
