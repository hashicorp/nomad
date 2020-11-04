package api

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
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

	streamCh, err := events.Stream(ctx, topics, 0, q)
	require.NoError(t, err)

	select {
	case event := <-streamCh:
		if event.Err != nil {
			require.Fail(t, err.Error())
		}
		require.Equal(t, len(event.Events), 1)
		require.Equal(t, "Eval", string(event.Events[0].Topic))
	case <-time.After(5 * time.Second):
		require.Fail(t, "failed waiting for event stream event")
	}
}

func TestEvent_Stream_Err_InvalidQueryParam(t *testing.T) {
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
		"Eval": {"::*"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = events.Stream(ctx, topics, 0, q)
	require.Error(t, err)
	require.Contains(t, err.Error(), "400")
	require.Contains(t, err.Error(), "Invalid key value pair")
}

func TestEvent_Stream_CloseCtx(t *testing.T) {
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

	streamCh, err := events.Stream(ctx, topics, 0, q)
	require.NoError(t, err)

	// cancel the request
	cancel()

	select {
	case event, ok := <-streamCh:
		require.False(t, ok)
		require.Nil(t, event)
	case <-time.After(5 * time.Second):
		require.Fail(t, "failed waiting for event stream event")
	}
}

func TestEventStream_PayloadValue(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
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
		"Node": {"*"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamCh, err := events.Stream(ctx, topics, 0, q)
	require.NoError(t, err)

	select {
	case event := <-streamCh:
		if event.Err != nil {
			require.NoError(t, err)
		}
		for _, e := range event.Events {
			if e.Node != nil {
				require.NotEqual(t, "", e.Node.ID)
				return
			} else {
				require.Fail(t, "expected either node or deployment to be set")
			}
		}
	case <-time.After(5 * time.Second):
		require.Fail(t, "failed waiting for event stream event")
	}
}
