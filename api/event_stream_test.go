package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/mitchellh/mapstructure"
	"github.com/shoenig/test/must"
)

func TestTopic_String(t *testing.T) {
	testutil.Parallel(t)

	testCases := []struct {
		inputTopic     Topic
		expectedOutput string
	}{
		{
			inputTopic:     TopicDeployment,
			expectedOutput: "Deployment",
		},
		{
			inputTopic:     TopicEvaluation,
			expectedOutput: "Evaluation",
		},
		{
			inputTopic:     TopicAllocation,
			expectedOutput: "Allocation",
		},
		{
			inputTopic:     TopicJob,
			expectedOutput: "Job",
		},
		{
			inputTopic:     TopicNode,
			expectedOutput: "Node",
		},
		{
			inputTopic:     TopicService,
			expectedOutput: "Service",
		},
		{
			inputTopic:     TopicAll,
			expectedOutput: "*",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expectedOutput, func(t *testing.T) {
			actualOutput := tc.inputTopic.String()
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestEvent_Stream(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// register job to generate events
	jobs := c.Jobs()
	job := testJob()
	resp2, _, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp2)

	// build event stream request
	events := c.EventStream()
	q := &QueryOptions{}
	topics := map[Topic][]string{
		TopicEvaluation: {"*"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamCh, err := events.Stream(ctx, topics, 0, q)
	must.NoError(t, err)

	select {
	case event := <-streamCh:
		if event.Err != nil {
			must.Unreachable(t, must.Sprintf("unexpected %v", event.Err))
		}
		must.Len(t, 1, event.Events)
		must.Eq(t, "Evaluation", string(event.Events[0].Topic))
	case <-time.After(5 * time.Second):
		must.Unreachable(t, must.Sprint("failed waiting for event stream event"))
	}
}

func TestEvent_Stream_Err_InvalidQueryParam(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// register job to generate events
	jobs := c.Jobs()
	job := testJob()
	resp2, _, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp2)

	// build event stream request
	events := c.EventStream()
	q := &QueryOptions{}
	topics := map[Topic][]string{
		TopicEvaluation: {"::*"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = events.Stream(ctx, topics, 0, q)
	must.ErrorContains(t, err, "Invalid key value pair")
}

func TestEvent_Stream_CloseCtx(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// register job to generate events
	jobs := c.Jobs()
	job := testJob()
	resp2, _, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp2)

	// build event stream request
	events := c.EventStream()
	q := &QueryOptions{}
	topics := map[Topic][]string{
		TopicEvaluation: {"*"},
	}

	ctx, cancel := context.WithCancel(context.Background())

	streamCh, err := events.Stream(ctx, topics, 0, q)
	must.NoError(t, err)

	// cancel the request
	cancel()

	select {
	case event, ok := <-streamCh:
		must.False(t, ok)
		must.Nil(t, event)
	case <-time.After(5 * time.Second):
		must.Unreachable(t, must.Sprint("failed waiting for event stream event"))
	}
}

func TestEventStream_PayloadValue(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()

	// register job to generate events
	jobs := c.Jobs()
	job := testJob()
	resp2, _, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp2)

	// build event stream request
	events := c.EventStream()
	q := &QueryOptions{}
	topics := map[Topic][]string{
		TopicNode: {"*"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamCh, err := events.Stream(ctx, topics, 0, q)
	must.NoError(t, err)

	select {
	case event := <-streamCh:
		if event.Err != nil {
			must.NoError(t, err)
		}
		for _, e := range event.Events {
			// verify that we get a node
			n, err := e.Node()
			must.NoError(t, err)
			must.UUIDv4(t, n.ID)

			// perform a raw decoding and look for:
			// - "ID" to make sure that raw decoding is working correctly
			// - "SecretID" to make sure it's not present
			raw := make(map[string]map[string]interface{}, 0)
			cfg := &mapstructure.DecoderConfig{
				Result: &raw,
			}
			dec, err := mapstructure.NewDecoder(cfg)
			must.NoError(t, err)
			must.NoError(t, dec.Decode(e.Payload))
			must.MapContainsKeys(t, raw, []string{"Node"})
			rawNode := raw["Node"]
			must.Eq(t, n.ID, rawNode["ID"].(string))
			must.Eq(t, "", rawNode["SecretID"])
		}
	case <-time.After(5 * time.Second):
		must.Unreachable(t, must.Sprint("failed waiting for event stream event"))
	}
}

func TestEventStream_PayloadValueHelpers(t *testing.T) {
	testutil.Parallel(t)

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
				must.NoError(t, err)
				must.Eq(t, TopicDeployment, event.Topic)

				d, err := event.Deployment()
				must.NoError(t, err)
				must.Eq(t, &Deployment{
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
				must.Eq(t, TopicEvaluation, event.Topic)
				eval, err := event.Evaluation()
				must.NoError(t, err)
				must.Eq(t, &Evaluation{
					ID:        "some-id",
					Namespace: "some-namespace-id",
				}, eval)
			},
		},
		{
			desc:  "allocation",
			input: []byte(`{"Topic": "Allocation", "Payload": {"Allocation":{"ID":"some-id","Namespace":"some-namespace-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				must.Eq(t, TopicAllocation, event.Topic)
				a, err := event.Allocation()
				must.NoError(t, err)
				must.Eq(t, &Allocation{
					ID:        "some-id",
					Namespace: "some-namespace-id",
				}, a)
			},
		},
		{
			input: []byte(`{"Topic": "Job", "Payload": {"Job":{"ID":"some-id","Namespace":"some-namespace-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				must.Eq(t, TopicJob, event.Topic)
				j, err := event.Job()
				must.NoError(t, err)
				must.Eq(t, &Job{
					ID:        pointerOf("some-id"),
					Namespace: pointerOf("some-namespace-id"),
				}, j)
			},
		},
		{
			desc:  "node",
			input: []byte(`{"Topic": "Node", "Payload": {"Node":{"ID":"some-id","Datacenter":"some-dc-id"}}}`),
			expectFn: func(t *testing.T, event Event) {
				must.Eq(t, TopicNode, event.Topic)
				n, err := event.Node()
				must.NoError(t, err)
				must.Eq(t, &Node{
					ID:         "some-id",
					Datacenter: "some-dc-id",
				}, n)
			},
		},
		{
			desc:  "service",
			input: []byte(`{"Topic": "Service", "Payload": {"Service":{"ID":"some-service-id","Namespace":"some-service-namespace-id","Datacenter":"us-east-1a"}}}`),
			expectFn: func(t *testing.T, event Event) {
				must.Eq(t, TopicService, event.Topic)
				a, err := event.Service()
				must.NoError(t, err)
				must.Eq(t, "us-east-1a", a.Datacenter)
				must.Eq(t, "some-service-id", a.ID)
				must.Eq(t, "some-service-namespace-id", a.Namespace)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var out Event
			err := json.Unmarshal(tc.input, &out)
			must.NoError(t, err)
			tc.expectFn(t, out)
		})
	}
}
