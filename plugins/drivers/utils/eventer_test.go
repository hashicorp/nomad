package utils

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/plugins/drivers/base"
	"github.com/stretchr/testify/require"
)

func TestEventer(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	stop := make(chan struct{})
	e := NewEventer(stop)

	events := []*base.TaskEvent{
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

	consumer1, err := e.TaskEvents(context.Background())
	require.NoError(err)
	consumer2, err := e.TaskEvents(context.Background())
	require.NoError(err)

	var buffer1, buffer2 []*base.TaskEvent
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			event, ok := <-consumer1
			if !ok {
				return
			}
			buffer1 = append(buffer1, event)
		}
	}()
	go func() {
		defer wg.Done()
		for {
			event, ok := <-consumer2
			if !ok {
				return
			}
			buffer2 = append(buffer2, event)
		}
	}()

	for _, event := range events {
		e.EmitEvent(event)
	}

	close(stop)
	wg.Wait()
	require.Exactly(events, buffer1)
	require.Exactly(events, buffer2)
}
