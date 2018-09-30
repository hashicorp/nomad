package utils

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
)

func TestEventer(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	e := NewEventer(ctx, testlog.HCLogger(t))

	events := []*drivers.TaskEvent{
		{
			TaskID:    "a",
			Timestamp: time.Now(),
		},
		{
			TaskID:    "b",
			Timestamp: time.Now(),
		},
		{
			TaskID:    "c",
			Timestamp: time.Now(),
		},
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	consumer1, err := e.TaskEvents(ctx1)
	require.NoError(err)
	ctx2 := (context.Background())
	consumer2, err := e.TaskEvents(ctx2)
	require.NoError(err)

	var buffer1, buffer2 []*drivers.TaskEvent
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		var i int
		for event := range consumer1 {
			i++
			buffer1 = append(buffer1, event)
			if i == 3 {
				break
			}
		}
	}()
	go func() {
		defer wg.Done()
		var i int
		for event := range consumer2 {
			i++
			buffer2 = append(buffer2, event)
			if i == 3 {
				break
			}
		}
	}()

	for _, event := range events {
		require.NoError(e.EmitEvent(event))
	}

	wg.Wait()
	require.Exactly(events, buffer1)
	require.Exactly(events, buffer2)
	cancel1()
	time.Sleep(100 * time.Millisecond)
	require.Equal(1, len(e.consumers))

	require.NoError(e.EmitEvent(&drivers.TaskEvent{}))
	ev, ok := <-consumer1
	require.Nil(ev)
	require.False(ok)
	ev, ok = <-consumer2
	require.NotNil(ev)
	require.True(ok)

	cancel()
	time.Sleep(100 * time.Millisecond)
	require.Zero(len(e.consumers))
	require.Error(e.EmitEvent(&drivers.TaskEvent{}))
}
