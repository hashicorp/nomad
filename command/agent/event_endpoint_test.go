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

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEvent struct {
	ID string
}

func TestEventStream(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
