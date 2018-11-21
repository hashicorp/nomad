package eventer

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
	defer cancel()
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
	defer cancel1()
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
			if i == len(events) {
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		var i int
		for event := range consumer2 {
			i++
			buffer2 = append(buffer2, event)
			if i == len(events) {
				return
			}
		}
	}()

	for _, event := range events {
		require.NoError(e.EmitEvent(event))
	}

	wg.Wait()
	require.Exactly(events, buffer1)
	require.Exactly(events, buffer2)
}

func TestEventer_iterateConsumers(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	e := &Eventer{
		events: make(chan *drivers.TaskEvent),
		ctx:    context.Background(),
		logger: testlog.HCLogger(t),
	}

	ev := &drivers.TaskEvent{
		TaskID:    "a",
		Timestamp: time.Now(),
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	consumer, err := e.TaskEvents(ctx1)
	require.NoError(err)
	require.Equal(1, len(e.consumers))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ev1, ok := <-consumer
		require.Exactly(ev, ev1)
		require.True(ok)
	}()
	e.iterateConsumers(ev)
	wg.Wait()

	go func() {
		cancel1()
		e.iterateConsumers(ev)
	}()
	ev1, ok := <-consumer
	require.False(ok)
	require.Nil(ev1)
	require.Equal(0, len(e.consumers))
}
