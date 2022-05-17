package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/hashicorp/nomad/nomad/state"
)

type testEvent struct {
	ID string
}

func TestEventStream(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, nil, func(s *TestAgent) {
		ctx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequestWithContext(ctx, "GET", "/v1/event/stream", nil)
		require.Nil(t, err)
		resp := httptest.NewRecorder()

		respErrCh := make(chan error)
		go func() {
			_, err = s.Server.EventStream(resp, req)
			respErrCh <- err
			assert.NoError(t, err)
		}()

		pub, err := s.Agent.server.State().EventBroker()
		require.NoError(t, err)
		pub.Publish(&structs.Events{Index: 100, Events: []structs.Event{{Payload: testEvent{ID: "123"}}}})

		testutil.WaitForResult(func() (bool, error) {
			got := resp.Body.String()
			want := `{"ID":"123"}`
			if strings.Contains(got, want) {
				return true, nil
			}

			return false, fmt.Errorf("missing expected json, got: %v, want: %v", got, want)
		}, func(err error) {
			cancel()
			require.Fail(t, err.Error())
		})

		// wait for response to close to prevent race between subscription
		// shutdown and server shutdown returning subscription closed by server err
		cancel()
		select {
		case err := <-respErrCh:
			require.Nil(t, err)
		case <-time.After(1 * time.Second):
			require.Fail(t, "waiting for request cancellation")
		}
	})
}

func TestEventStream_NamespaceQuery(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, nil, func(s *TestAgent) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", "/v1/event/stream?namespace=foo", nil)
		require.Nil(t, err)
		resp := httptest.NewRecorder()

		respErrCh := make(chan error)
		go func() {
			_, err = s.Server.EventStream(resp, req)
			respErrCh <- err
			assert.NoError(t, err)
		}()

		pub, err := s.Agent.server.State().EventBroker()
		require.NoError(t, err)

		badID := uuid.Generate()
		pub.Publish(&structs.Events{Index: 100, Events: []structs.Event{{Namespace: "bar", Payload: testEvent{ID: badID}}}})
		pub.Publish(&structs.Events{Index: 101, Events: []structs.Event{{Namespace: "foo", Payload: testEvent{ID: "456"}}}})

		testutil.WaitForResult(func() (bool, error) {
			got := resp.Body.String()
			want := `"Namespace":"foo"`
			if strings.Contains(got, badID) {
				return false, fmt.Errorf("expected non matching namespace to be filtered, got:%v", got)
			}
			if strings.Contains(got, want) {
				return true, nil
			}

			return false, fmt.Errorf("missing expected json, got: %v, want: %v", got, want)
		}, func(err error) {
			require.Fail(t, err.Error())
		})

		// wait for response to close to prevent race between subscription
		// shutdown and server shutdown returning subscription closed by server err
		cancel()
		select {
		case err := <-respErrCh:
			require.Nil(t, err)
		case <-time.After(1 * time.Second):
			require.Fail(t, "waiting for request cancellation")
		}
	})
}

func TestEventStream_QueryParse(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		desc    string
		query   string
		want    map[structs.Topic][]string
		wantErr bool
	}{
		{
			desc:  "all topics and keys specified",
			query: "?topic=*:*",
			want: map[structs.Topic][]string{
				"*": {"*"},
			},
		},
		{
			desc:  "all topics and keys inferred",
			query: "",
			want: map[structs.Topic][]string{
				"*": {"*"},
			},
		},
		{
			desc:    "invalid key value formatting",
			query:   "?topic=NodeDrain:*:*",
			wantErr: true,
		},
		{
			desc:    "Infer wildcard if absent",
			query:   "?topic=NodeDrain",
			wantErr: false,
			want: map[structs.Topic][]string{
				"NodeDrain": {"*"},
			},
		},
		{
			desc:  "single topic and key",
			query: "?topic=NodeDrain:*",
			want: map[structs.Topic][]string{
				"NodeDrain": {"*"},
			},
		},
		{
			desc:  "single topic multiple keys",
			query: "?topic=NodeDrain:*&topic=NodeDrain:3caace09-f1f4-4d23-b37a-9ab5eb75069d",
			want: map[structs.Topic][]string{
				"NodeDrain": {
					"*",
					"3caace09-f1f4-4d23-b37a-9ab5eb75069d",
				},
			},
		},
		{
			desc:  "multiple topics",
			query: "?topic=NodeRegister:*&topic=NodeDrain:3caace09-f1f4-4d23-b37a-9ab5eb75069d",
			want: map[structs.Topic][]string{
				"NodeDrain": {
					"3caace09-f1f4-4d23-b37a-9ab5eb75069d",
				},
				"NodeRegister": {
					"*",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			raw := fmt.Sprintf("http://localhost:80/v1/events%s", tc.query)
			req, err := url.Parse(raw)
			require.NoError(t, err)

			got, err := parseEventTopics(req.Query())
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestHTTP_AllocPort_Parsing(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, nil, func(srv *TestAgent) {
		client := srv.Client()
		defer srv.Shutdown()
		defer client.Close()

		testutil.WaitForLeader(t, srv.Agent.RPC)
		testutil.WaitForClient(t, srv.Agent.Client().RPC, srv.Agent.Client().NodeID(), srv.Agent.Client().Region())

		job := mock.Job()
		job.Constraints = nil
		job.TaskGroups[0].Constraints = nil
		job.TaskGroups[0].Count = 1
		job.TaskGroups[0].Tasks[0].Resources.Networks = append(job.TaskGroups[0].Tasks[0].Resources.Networks, &structs.NetworkResource{
			ReservedPorts: []structs.Port{
				{
					Label: "static",
					To:    5000,
				},
			},
		})

		registerReq := &structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var registerResp structs.JobRegisterResponse
		require.Nil(t, srv.Agent.RPC("Job.Register", registerReq, &registerResp))

		// TODO:
		// - Get Job from API to prove static port not lost
		// - Update the state to prove event stream works
		// - Try to actually get the alloc to run

		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = srv.client.NodeID()

		require.Nil(t, srv.server.State().UpsertJobSummary(101, mock.JobSummary(alloc.JobID))

		err = state.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{alloc1})
		require.Nil(err)

		// Ensure allocation gets upserted with desired status.
		var alloc *structs.Allocation
		testutil.WaitForResult(func() (bool, error) {
			allocs, err := srv.server.State().AllocsByJob(nil, "", job.ID, true)
			if err != nil {
				return false, err
			}
			for _, a := range allocs {
				alloc = a
				return true, nil
			}
			return false, nil
		}, func(err error) {
			require.NoError(t, err, "allocation query failed")
		})

		require.NotNil(t, alloc)

		alloc.NodeID = srv.client.NodeID()
		alloc.ClientStatus = structs.AllocClientStatusRunning
		require.Nil(t, srv.Agent.server.State().UpsertAllocs(structs.MsgTypeTestSetup, 10, []*structs.Allocation{alloc}))

		topics := map[api.Topic][]string{
			api.TopicAllocation: {job.ID},
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		events := client.EventStream()
		streamCh, err := events.Stream(ctx, topics, 1, nil)
		require.NoError(t, err)

		var allocEvents []api.Event
		// gather job alloc events
		go func() {
			for {
				select {
				case event, ok := <-streamCh:
					if !ok {
						return
					}
					if event.IsHeartbeat() {
						continue
					}
					allocEvents = append(allocEvents, event.Events...)
				case <-time.After(5 * time.Second):
					require.Fail(t, "failed waiting for event stream event")
				}
			}
		}()

		var eventAlloc *api.Allocation
		testutil.WaitForResult(func() (bool, error) {
			var got string
			for _, e := range allocEvents {
				t.Logf("event_type: %s", e.Type)
				if e.Type == structs.TypeAllocationCreated || e.Type == structs.TypeAllocationUpdated {
					eventAlloc, err = e.Allocation()
					return true, nil
				}
				got = e.Type
			}
			return false, fmt.Errorf("expected to receive allocation updated event, got: %#v", got)
		}, func(e error) {
			require.NoError(t, err)
		})

		require.NotNil(t, eventAlloc)

		networkResource := eventAlloc.AllocatedResources.Tasks["web"].Networks[0]
		require.Equal(t, 5000, networkResource.ReservedPorts[0].Value)
		require.NotEqual(t, 0, networkResource.DynamicPorts[0].Value)
	})
}
