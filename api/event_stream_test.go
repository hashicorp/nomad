package api

import (
	"context"
	"encoding/json"
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
		TopicEvaluation: {"*"},
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
		require.Equal(t, "Evaluation", string(event.Events[0].Topic))
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
		TopicEvaluation: {"::*"},
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
		TopicEvaluation: {"*"},
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
		TopicNode: {"*"},
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
			n, err := e.Node()
			require.NoError(t, err)
			require.NotEqual(t, "", n.ID)
		}
	case <-time.After(5 * time.Second):
		require.Fail(t, "failed waiting for event stream event")
	}
}

func TestEventStream_PayloadValueHelpers(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		event    Event
		input    []byte
		err      string
		expectFn func(t *testing.T, event Event)
	}{
		{
			desc:  "deployment",
			input: []byte(`{"Topic": "Deployment", "Payload": {"Deployment":{"ID":"some-id","JobID":"some-job-id", "TaskGroups": {"tg1": {"RequireProgressBy": "2020-11-05T11:52:54.370774000-05:00"}}}}}`),
			expectFn: func(t *testing.T, event Event) {
				eventTime, err := time.Parse(time.RFC3339, "2020-11-05T11:52:54.370774000-05:00")
				require.NoError(t, err)
				require.Equal(t, TopicDeployment, event.Topic)

				d, err := event.Deployment()
				require.NoError(t, err)
				require.NoError(t, err)
				require.Equal(t, &Deployment{
					ID:    "some-id",
					JobID: "some-job-id",
					TaskGroups: map[string]*DeploymentState{
						"tg1": {
							RequireProgressBy: eventTime,
						},
					},
				}, d)
			},
		},
		{
			desc:  "evaluation",
			input: []byte(`{"Topic": "Evaluation", "Payload": {"Evaluation":{"ID":"some-id","Namespace":"some-namespace-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, TopicEvaluation, event.Topic)
				eval, err := event.Evaluation()
				require.NoError(t, err)

				require.Equal(t, &Evaluation{
					ID:        "some-id",
					Namespace: "some-namespace-id",
				}, eval)
			},
		},
		{
			desc:  "allocation",
			input: []byte(`{"Topic": "Allocation", "Payload": {"Allocation":{"ID":"some-id","Namespace":"some-namespace-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, TopicAllocation, event.Topic)
				a, err := event.Allocation()
				require.NoError(t, err)
				require.Equal(t, &Allocation{
					ID:        "some-id",
					Namespace: "some-namespace-id",
				}, a)
			},
		},
		{
			input: []byte(`{"Topic": "Job", "Payload": {"Job":{"ID":"some-id","Namespace":"some-namespace-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, TopicJob, event.Topic)
				j, err := event.Job()
				require.NoError(t, err)
				require.Equal(t, &Job{
					ID:        stringToPtr("some-id"),
					Namespace: stringToPtr("some-namespace-id"),
				}, j)
			},
		},
		{
			desc:  "node",
			input: []byte(`{"Topic": "Node", "Payload": {"Node":{"ID":"some-id","Datacenter":"some-dc-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				require.Equal(t, TopicNode, event.Topic)
				n, err := event.Node()
				require.NoError(t, err)
				require.Equal(t, &Node{
					ID:         "some-id",
					Datacenter: "some-dc-id",
				}, n)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var out Event
			err := json.Unmarshal(tc.input, &out)
			require.NoError(t, err)
			tc.expectFn(t, out)
		})
	}
}
