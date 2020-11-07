package nomad

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
)

func TestEventStream(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.EnableEventBroker = true
	})
	defer cleanupS1()

	// Create request for all topics and keys
	req := structs.EventStreamRequest{
		Topics: map[structs.Topic][]string{"*": {"*"}},
		QueryOptions: structs.QueryOptions{
			Region: s1.Region(),
		},
	}

	handler, err := s1.StreamingRpcHandler("Event.Stream")
	require.Nil(t, err)

	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	streamMsg := make(chan *structs.EventStreamWrapper)

	// invoke handler
	go handler(p2)

	// decode request responses
	go func() {
		decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
		for {
			var msg structs.EventStreamWrapper
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "closed") {
					return
				}
				errCh <- fmt.Errorf("error decoding: %w", err)
			}

			streamMsg <- &msg
		}
	}()

	// retrieve publisher for server, send event
	publisher, err := s1.State().EventBroker()
	require.NoError(t, err)

	node := mock.Node()
	publisher.Publish(&structs.Events{Index: uint64(1), Events: []structs.Event{{Topic: "test", Payload: node}}})

	// Send request
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	require.Nil(t, encoder.Encode(req))

	publisher.Publish(&structs.Events{Index: uint64(2), Events: []structs.Event{{Topic: "test", Payload: node}}})
	publisher.Publish(&structs.Events{Index: uint64(3), Events: []structs.Event{{Topic: "test", Payload: node}}})

	timeout := time.After(3 * time.Second)
	got := 0
	want := 3
OUTER:
	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for event stream")
		case err := <-errCh:
			t.Fatal(err)
		case msg := <-streamMsg:
			if msg.Error != nil {
				t.Fatalf("Got error: %v", msg.Error.Error())
			}

			// ignore heartbeat
			if msg.Event == stream.JsonHeartbeat {
				continue
			}

			var event structs.Events
			err = json.Unmarshal(msg.Event.Data, &event)
			require.NoError(t, err)

			// decode fully to ensure we received expected out
			var out structs.Node
			cfg := &mapstructure.DecoderConfig{
				Metadata: nil,
				Result:   &out,
			}
			dec, err := mapstructure.NewDecoder(cfg)
			dec.Decode(event.Events[0].Payload)
			require.NoError(t, err)
			require.Equal(t, node.ID, out.ID)

			got++
			if got == want {
				break OUTER
			}
		}
	}
}

// TestEventStream_StreamErr asserts an error is returned when an event publisher
// closes its subscriptions
func TestEventStream_StreamErr(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.EnableEventBroker = true
	})
	defer cleanupS1()

	testutil.WaitForLeader(t, s1.RPC)

	req := structs.EventStreamRequest{
		Topics: map[structs.Topic][]string{"*": {"*"}},
		QueryOptions: structs.QueryOptions{
			Region: s1.Region(),
		},
	}

	handler, err := s1.StreamingRpcHandler("Event.Stream")
	require.Nil(t, err)

	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	streamMsg := make(chan *structs.EventStreamWrapper)

	go handler(p2)

	go func() {
		decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
		for {
			var msg structs.EventStreamWrapper
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "closed") {
					return
				}
				errCh <- fmt.Errorf("error decoding: %w", err)
			}

			streamMsg <- &msg
		}
	}()

	publisher, err := s1.State().EventBroker()
	require.NoError(t, err)

	node := mock.Node()

	// send req
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	require.Nil(t, encoder.Encode(req))

	// publish some events
	publisher.Publish(&structs.Events{Index: uint64(1), Events: []structs.Event{{Topic: "test", Payload: node}}})
	publisher.Publish(&structs.Events{Index: uint64(2), Events: []structs.Event{{Topic: "test", Payload: node}}})

	timeout := time.After(5 * time.Second)
OUTER:
	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for event stream")
		case err := <-errCh:
			t.Fatal(err)
		case msg := <-streamMsg:
			// close the publishers subscriptions forcing an error
			// after an initial event is received
			publisher.CloseAll()
			if msg.Error == nil {
				// continue trying for error
				continue
			}
			require.NotNil(t, msg.Error)
			require.Contains(t, msg.Error.Error(), "subscription closed by server")
			break OUTER
		}
	}
}

// TestEventStream_RegionForward tests event streaming from one server
// to another in a different region
func TestEventStream_RegionForward(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.EnableEventBroker = true
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.EnableEventBroker = true
		c.Region = "foo"
	})
	defer cleanupS2()

	TestJoin(t, s1, s2)

	// Create request targed for region foo
	req := structs.EventStreamRequest{
		Topics: map[structs.Topic][]string{"*": {"*"}},
		QueryOptions: structs.QueryOptions{
			Region: "foo",
		},
	}

	// Query s1 handler
	handler, err := s1.StreamingRpcHandler("Event.Stream")
	require.Nil(t, err)

	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	streamMsg := make(chan *structs.EventStreamWrapper)

	go handler(p2)

	go func() {
		decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
		for {
			var msg structs.EventStreamWrapper
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "closed") {
					return
				}
				errCh <- fmt.Errorf("error decoding: %w", err)
			}

			streamMsg <- &msg
		}
	}()

	// publish with server 2
	publisher, err := s2.State().EventBroker()
	require.NoError(t, err)

	node := mock.Node()
	publisher.Publish(&structs.Events{Index: uint64(1), Events: []structs.Event{{Topic: "test", Payload: node}}})

	// send req
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	require.Nil(t, encoder.Encode(req))

	timeout := time.After(3 * time.Second)
OUTER:
	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for event stream")
		case err := <-errCh:
			t.Fatal(err)
		case msg := <-streamMsg:
			if msg.Error != nil {
				t.Fatalf("Got error: %v", msg.Error.Error())
			}

			if msg.Event == stream.JsonHeartbeat {
				continue
			}

			var event structs.Events
			err = json.Unmarshal(msg.Event.Data, &event)
			require.NoError(t, err)

			var out structs.Node
			cfg := &mapstructure.DecoderConfig{
				Metadata: nil,
				Result:   &out,
			}
			dec, err := mapstructure.NewDecoder(cfg)
			dec.Decode(event.Events[0].Payload)
			require.NoError(t, err)
			require.Equal(t, node.ID, out.ID)
			break OUTER
		}
	}
}

func TestEventStream_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// start server
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyNsGood := mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob})
	tokenNsFoo := mock.CreatePolicyAndToken(t, s.State(), 1006, "valid", policyNsGood)

	policyNsNode := mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob})
	policyNsNode += "\n" + mock.NodePolicy("read")
	tokenNsNode := mock.CreatePolicyAndToken(t, s.State(), 1007, "validnNsNode", policyNsNode)

	cases := []struct {
		Name        string
		Token       string
		Topics      map[structs.Topic][]string
		Namespace   string
		ExpectedErr string
		PublishFn   func(p *stream.EventBroker)
	}{
		{
			Name:  "no token",
			Token: "",
			Topics: map[structs.Topic][]string{
				"*": {"*"},
			},
			ExpectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:  "bad token",
			Token: tokenBad.SecretID,
			Topics: map[structs.Topic][]string{
				"*": {"*"},
			},
			ExpectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:  "root token",
			Token: root.SecretID,
			Topics: map[structs.Topic][]string{
				"*": {"*"},
			},
			ExpectedErr: "subscription closed by server",
		},
		{
			Name:  "job namespace token - correct ns",
			Token: tokenNsFoo.SecretID,
			Topics: map[structs.Topic][]string{
				"Job":        {"*"},
				"Eval":       {"*"},
				"Alloc":      {"*"},
				"Deployment": {"*"},
			},
			Namespace:   "foo",
			ExpectedErr: "subscription closed by server",
			PublishFn: func(p *stream.EventBroker) {
				p.Publish(&structs.Events{Index: uint64(1000), Events: []structs.Event{{Topic: "Job", Namespace: "foo", Payload: mock.Job()}}})
			},
		},
		{
			Name:  "job namespace token - incorrect ns",
			Token: tokenNsFoo.SecretID,
			Topics: map[structs.Topic][]string{
				"Job": {"*"}, // good
			},
			Namespace:   "bar", // bad
			ExpectedErr: structs.ErrPermissionDenied.Error(),
			PublishFn: func(p *stream.EventBroker) {
				p.Publish(&structs.Events{Index: uint64(1000), Events: []structs.Event{{Topic: "Job", Namespace: "foo", Payload: mock.Job()}}})
			},
		},
		{
			Name:  "job namespace token - request management topic",
			Token: tokenNsFoo.SecretID,
			Topics: map[structs.Topic][]string{
				"*": {"*"}, // bad
			},
			Namespace:   "foo",
			ExpectedErr: structs.ErrPermissionDenied.Error(),
			PublishFn: func(p *stream.EventBroker) {
				p.Publish(&structs.Events{Index: uint64(1000), Events: []structs.Event{{Topic: "Job", Namespace: "foo", Payload: mock.Job()}}})
			},
		},
		{
			Name:  "job namespace token - request invalid node topic",
			Token: tokenNsFoo.SecretID,
			Topics: map[structs.Topic][]string{
				"Eval": {"*"}, // good
				"Node": {"*"}, // bad
			},
			Namespace:   "foo",
			ExpectedErr: structs.ErrPermissionDenied.Error(),
			PublishFn: func(p *stream.EventBroker) {
				p.Publish(&structs.Events{Index: uint64(1000), Events: []structs.Event{{Topic: "Job", Namespace: "foo", Payload: mock.Job()}}})
			},
		},
		{
			Name:  "job+node namespace token, valid",
			Token: tokenNsNode.SecretID,
			Topics: map[structs.Topic][]string{
				"Eval": {"*"}, // good
				"Node": {"*"}, // good
			},
			Namespace:   "foo",
			ExpectedErr: "subscription closed by server",
			PublishFn: func(p *stream.EventBroker) {
				p.Publish(&structs.Events{Index: uint64(1000), Events: []structs.Event{{Topic: "Node", Payload: mock.Node()}}})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			var ns string
			if tc.Namespace != "" {
				ns = tc.Namespace
			}
			// Create request for all topics and keys
			req := structs.EventStreamRequest{
				Topics: tc.Topics,
				QueryOptions: structs.QueryOptions{
					Region:    s.Region(),
					Namespace: ns,
					AuthToken: tc.Token,
				},
			}

			handler, err := s.StreamingRpcHandler("Event.Stream")
			require.Nil(err)

			// create pipe
			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			errCh := make(chan error)
			streamMsg := make(chan *structs.EventStreamWrapper)

			go handler(p2)

			// Start decoder
			go func() {
				decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
				for {
					var msg structs.EventStreamWrapper
					if err := decoder.Decode(&msg); err != nil {
						if err == io.EOF || strings.Contains(err.Error(), "closed") {
							return
						}
						errCh <- fmt.Errorf("error decoding: %w", err)
					}

					streamMsg <- &msg
				}
			}()

			// send request
			encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
			require.Nil(encoder.Encode(req))

			publisher, err := s.State().EventBroker()
			require.NoError(err)

			// publish some events
			node := mock.Node()

			publisher.Publish(&structs.Events{Index: uint64(1), Events: []structs.Event{{Topic: "test", Payload: node}}})
			publisher.Publish(&structs.Events{Index: uint64(2), Events: []structs.Event{{Topic: "test", Payload: node}}})

			if tc.PublishFn != nil {
				tc.PublishFn(publisher)
			}

			timeout := time.After(5 * time.Second)
		OUTER:
			for {
				select {
				case <-timeout:
					t.Fatal("timeout waiting for events")
				case err := <-errCh:
					t.Fatal(err)
				case msg := <-streamMsg:
					// force error by closing all subscriptions
					publisher.CloseAll()
					if msg.Error == nil {
						continue
					}

					if strings.Contains(msg.Error.Error(), tc.ExpectedErr) {
						break OUTER
					} else {
						t.Fatalf("unexpected error %v", msg.Error)
					}
				}
			}
		})
	}
}

func TestEvent_UpdateSinks(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	sink := mock.EventSink()

	require.NoError(t, s1.fsm.State().UpsertEventSink(1000, sink))

	// request sink doesn't need to be pointer
	s := &structs.EventSink{
		ID:          sink.ID,
		LatestIndex: uint64(300),
	}

	req := &structs.EventSinkProgressRequest{
		Sinks:        []*structs.EventSink{s},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var resp structs.GenericResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Event.UpdateSinks", req, &resp))
	require.NotEqual(t, 0, resp.Index)

	// Check for the sink in the FSM

	state := s1.fsm.State()
	out, err := state.EventSinkByID(nil, sink.ID)
	require.NoError(t, err)

	require.Equal(t, s.LatestIndex, out.LatestIndex)
}

func TestEvent_UpsertSink(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	sink := mock.EventSink()

	req := &structs.EventSinkUpsertRequest{
		Sink:         sink,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var resp structs.GenericResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Event.UpsertSink", req, &resp))
	require.NotEqual(t, 0, resp.Index)

	// Check for the sink in the FSM
	state := s1.fsm.State()
	out, err := state.EventSinkByID(nil, sink.ID)
	require.NoError(t, err)

	// set the index so we can compare values
	sink.CreateIndex = resp.Index
	sink.ModifyIndex = resp.Index

	require.EqualValues(t, sink, out)
}

func TestEvent_UpsertSink_Invalid(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	sink := &structs.EventSink{
		Type: structs.SinkWebhook,
	}

	req := &structs.EventSinkUpsertRequest{
		Sink:         sink,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "Event.UpsertSink", req, &resp)
	require.Error(t, err)

	require.Contains(t, err.Error(), "Missing sink ID")
	require.Contains(t, err.Error(), "Webhook sink requires a valid Address")
}

func TestEvent_GetSink(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	sink := mock.EventSink()

	require.NoError(t, s1.fsm.State().UpsertEventSink(1000, sink))

	get := &structs.EventSinkSpecificRequest{
		ID: sink.ID,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.EventSinkResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Event.GetSink", get, &resp))
	require.EqualValues(t, 1000, resp.Index)
	require.Equal(t, sink.ID, resp.Sink.ID)

	// Query for a non-existent sink
	get.ID = uuid.Generate()
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Event.GetSink", get, &resp))

	require.EqualValues(t, 1000, resp.Index)
	require.Nil(t, resp.Sink)
}

func TestEvent_DeleteSink(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	sink := mock.EventSink()

	require.NoError(t, s1.fsm.State().UpsertEventSink(1000, sink))

	get := &structs.EventSinkDeleteRequest{
		IDs: []string{sink.ID},
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}

	var resp structs.GenericResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Event.DeleteSink", get, &resp))
	require.NotEqual(t, uint64(0), resp.Index)

	state := s1.fsm.State()
	out, err := state.EventSinkByID(nil, sink.ID)
	require.NoError(t, err)
	require.Nil(t, out)
}

func TestEvent_ListSinks(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	sink := mock.EventSink()
	sink2 := mock.EventSink()

	require.NoError(t, s1.fsm.State().UpsertEventSink(1000, sink))
	require.NoError(t, s1.fsm.State().UpsertEventSink(1001, sink2))

	get := &structs.EventSinkListRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.EventSinkListResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Event.ListSinks", get, &resp))
	require.Len(t, resp.Sinks, 2)

}
