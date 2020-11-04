package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestEvent_Unmarshal(t *testing.T) {
	input := []byte(`{"Index": 1, "Payload": { "Deployment": {"ID": "TEST" }}}`)

	var e Event
	err := json.Unmarshal(input, &e)
	require.NoError(t, err)
	spew.Dump(e)
}

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

func TestEventStream_SetPayloadValue(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		topic    Topic
		event    Event
		input    []byte
		err      string
		expectFn func(t *testing.T, event Event)
	}{
		{
			input: []byte(`{"Topic": "Deployment", "Payload": {"Deployment":{"ID":"some-id","JobID":"some-job-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, TopicDeployment, event.Topic)
				require.Equal(t, &Deployment{
					ID:    "some-id",
					JobID: "some-job-id",
				}, event.Deployment)
			},
		},
		{
			input: []byte(`{"Topic": "Eval", "Payload": {"Eval":{"ID":"some-id","Namespace":"some-namespace-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, TopicEval, event.Topic)
				require.Equal(t, &Evaluation{
					ID:        "some-id",
					Namespace: "some-namespace-id",
				}, event.Evaluation)
			},
		},
		{
			input: []byte(`{"Topic": "Alloc", "Payload": {"Alloc":{"ID":"some-id","Namespace":"some-namespace-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, TopicAlloc, event.Topic)
				require.Equal(t, &Allocation{
					ID:        "some-id",
					Namespace: "some-namespace-id",
				}, event.Allocation)
			},
		},
		{
			input: []byte(`{"Topic": "Job", "Payload": {"Job":{"ID":"some-id","Namespace":"some-namespace-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, TopicJob, event.Topic)
				require.Equal(t, &Job{
					ID:        stringToPtr("some-id"),
					Namespace: stringToPtr("some-namespace-id"),
				}, event.Job)
			},
		},
		{
			input: []byte(`{"Topic": "Node", "Payload": {"Node":{"ID":"some-id","Datacenter":"some-dc-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, TopicNode, event.Topic)
				require.Equal(t, &Node{
					ID:         "some-id",
					Datacenter: "some-dc-id",
				}, event.Node)
			},
		},
		{
			input: []byte(`{"Topic": "unknown", "Payload": {"Pod":{"Foo": "invalid"}}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, Topic("unknown"), event.Topic)
			},
		},
		{
			input: []byte(`{"Topic": "*", "Payload": {}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, TopicAll, event.Topic)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.topic), func(t *testing.T) {
			var out Event
			err := json.Unmarshal(tc.input, &out)
			require.NoError(t, err)
			tc.expectFn(t, out)
		})
	}
}
