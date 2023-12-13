// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventBufferFuzz(t *testing.T) {
	ci.Parallel(t)

	nReaders := 1000
	nMessages := 1000

	b := newEventBuffer(1000)

	// Start a write goroutine that will publish 10000 messages with sequential
	// indexes and some jitter in timing (to allow clients to "catch up" and block
	// waiting for updates).
	go func() {
		seed := time.Now().UnixNano()
		t.Logf("Using seed %d", seed)
		// z is a Zipfian distribution that gives us a number of milliseconds to
		// sleep which are mostly low - near zero but occasionally spike up to near
		// 100.
		z := rand.NewZipf(rand.New(rand.NewSource(seed)), 1.5, 1.5, 50)

		for i := 0; i < nMessages; i++ {
			// Event content is arbitrary and not valid for our use of buffers in
			// streaming - here we only care about the semantics of the buffer.
			e := structs.Event{
				Index: uint64(i), // Indexes should be contiguous
			}
			b.Append(&structs.Events{Index: uint64(i), Events: []structs.Event{e}})
			// Sleep sometimes for a while to let some subscribers catch up
			wait := time.Duration(z.Uint64()) * time.Millisecond
			time.Sleep(wait)
		}
	}()

	// Run n subscribers following and verifying
	errCh := make(chan error, nReaders)

	// Load head here so all subscribers start from the same point or they might
	// not run until several appends have already happened.
	head := b.Head()

	for i := 0; i < nReaders; i++ {
		go func(i int) {
			expect := uint64(0)
			item := head
			var err error
			for {
				item, err = item.Next(context.Background(), nil)
				if err != nil {
					errCh <- fmt.Errorf("subscriber %05d failed getting next %d: %s", i,
						expect, err)
					return
				}
				if item.Events.Events[0].Index != expect {
					errCh <- fmt.Errorf("subscriber %05d got bad event want=%d, got=%d", i,
						expect, item.Events.Events[0].Index)
					return
				}
				expect++
				if expect == uint64(nMessages) {
					// Succeeded
					errCh <- nil
					return
				}
			}
		}(i)
	}

	// Wait for all readers to finish one way or other
	for i := 0; i < nReaders; i++ {
		err := <-errCh
		assert.NoError(t, err)
	}
}

func TestEventBuffer_Slow_Reader(t *testing.T) {
	ci.Parallel(t)

	b := newEventBuffer(10)

	for i := 1; i < 11; i++ {
		e := structs.Event{
			Index: uint64(i), // Indexes should be contiguous
		}
		b.Append(&structs.Events{Index: uint64(i), Events: []structs.Event{e}})
	}

	require.Equal(t, 10, b.Len())

	head := b.Head()

	for i := 10; i < 15; i++ {
		e := structs.Event{
			Index: uint64(i), // Indexes should be contiguous
		}
		b.Append(&structs.Events{Index: uint64(i), Events: []structs.Event{e}})
	}

	// Ensure the slow reader errors to handle dropped events and
	// fetch latest head
	ev, err := head.Next(context.Background(), nil)
	require.Error(t, err)
	require.Nil(t, ev)

	newHead := b.Head()
	require.Equal(t, 5, int(newHead.Events.Index))
}

func TestEventBuffer_Size(t *testing.T) {
	ci.Parallel(t)

	b := newEventBuffer(100)

	for i := 0; i < 10; i++ {
		e := structs.Event{
			Index: uint64(i), // Indexes should be contiguous
		}
		b.Append(&structs.Events{Index: uint64(i), Events: []structs.Event{e}})
	}

	require.Equal(t, 10, b.Len())
}

func TestEventBuffer_MaxSize(t *testing.T) {
	ci.Parallel(t)

	b := newEventBuffer(10)

	var events []structs.Event
	for i := 0; i < 100; i++ {
		events = append(events, structs.Event{})
	}

	b.Append(&structs.Events{Index: uint64(1), Events: events})
	require.Equal(t, 1, b.Len())
}

// TestEventBuffer_Emptying_Buffer tests the behavior when all items
// are removed, the event buffer should advance its head down to the last message
// and insert a placeholder sentinel value.
func TestEventBuffer_Emptying_Buffer(t *testing.T) {
	ci.Parallel(t)

	b := newEventBuffer(10)

	for i := 0; i < 10; i++ {
		e := structs.Event{
			Index: uint64(i), // Indexes should be contiguous
		}
		b.Append(&structs.Events{Index: uint64(i), Events: []structs.Event{e}})
	}

	require.Equal(t, 10, int(b.Len()))

	// empty the buffer, which will bring the event buffer down
	// to a single sentinel value
	for i := 0; i < 16; i++ {
		b.advanceHead()
	}

	// head and tail are now a sentinel value
	head := b.Head()
	tail := b.Tail()
	require.Equal(t, 0, int(head.Events.Index))
	require.Equal(t, 0, b.Len())
	require.Equal(t, head, tail)

	e := structs.Event{
		Index: uint64(100),
	}
	b.Append(&structs.Events{Index: uint64(100), Events: []structs.Event{e}})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))
	defer cancel()

	next, err := head.Next(ctx, make(chan struct{}))
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, uint64(100), next.Events.Index)

}

func TestEventBuffer_StartAt_CurrentIdx_Past_Start(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		desc     string
		req      uint64
		expected uint64
		offset   int
	}{
		{
			desc:     "requested index less than head receives head",
			req:      10,
			expected: 11,
			offset:   1,
		},
		{
			desc:     "requested exact match head",
			req:      11,
			expected: 11,
			offset:   0,
		},
		{
			desc:     "requested exact match",
			req:      42,
			expected: 42,
			offset:   0,
		},
		{
			desc:     "requested index greater than tail receives tail",
			req:      500,
			expected: 100,
			offset:   400,
		},
	}

	// buffer starts at index 11 goes to 100
	b := newEventBuffer(100)

	for i := 11; i <= 100; i++ {
		e := structs.Event{
			Index: uint64(i), // Indexes should be contiguous
		}
		b.Append(&structs.Events{Index: uint64(i), Events: []structs.Event{e}})
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got, offset := b.StartAtClosest(tc.req)
			require.Equal(t, int(tc.expected), int(got.Events.Index))
			require.Equal(t, tc.offset, offset)
		})
	}
}
