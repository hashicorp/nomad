package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEvent_Stream(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// register job to generate events
	jobs := c.Jobs()
	job := testJob()
	resp2, _, err := jobs.Register(job, nil)
	require.Nil(t, err)
	require.NotNil(t, resp2)

	// build event stream request
	events := c.EventStream()
	q := &QueryOptions{}
	topics := map[Topic][]string{
		"Eval": {"*"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamCh, errCh := events.Stream(ctx, topics, 0, q)

OUTER:
	for {
		select {
		case event := <-streamCh:
			require.Equal(t, len(event.Events), 1)
			require.Equal(t, "Eval", string(event.Events[0].Topic))

			break OUTER
		case err := <-errCh:
			require.Fail(t, err.Error())
		case <-time.After(5 * time.Second):
			require.Fail(t, "failed waiting for event stream event")
		}
	}
}
