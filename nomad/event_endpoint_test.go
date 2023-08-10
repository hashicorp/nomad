// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"bytes"
	"context"
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
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/mapstructure"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestEventStream(t *testing.T) {
	ci.Parallel(t)

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
			if bytes.Equal(msg.Event.Data, stream.JsonHeartbeat.Data) {
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
	ci.Parallel(t)

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
	ci.Parallel(t)

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

			if bytes.Equal(msg.Event.Data, stream.JsonHeartbeat.Data) {
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
	ci.Parallel(t)
	require := require.New(t)

	// start server
	s, _, cleanupS := TestACLServer(t, nil)
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
				structs.TopicAll: {"*"},
			},
			ExpectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:  "bad token",
			Token: tokenBad.SecretID,
			Topics: map[structs.Topic][]string{
				structs.TopicAll: {"*"},
			},
			ExpectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:  "job namespace token - correct ns",
			Token: tokenNsFoo.SecretID,
			Topics: map[structs.Topic][]string{
				structs.TopicJob:        {"*"},
				structs.TopicEvaluation: {"*"},
				structs.TopicAllocation: {"*"},
				structs.TopicDeployment: {"*"},
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
				structs.TopicJob: {"*"}, // good
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
				structs.TopicAll: {"*"}, // bad
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
				structs.TopicEvaluation: {"*"}, // good
				structs.TopicNode:       {"*"}, // bad
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
				structs.TopicEvaluation: {"*"}, // good
				structs.TopicNode:       {"*"}, // good
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

// TestEventStream_ACL_Update_Close_Stream asserts that an active subscription
// is closed after the token is no longer valid
func TestEventStream_ACL_Update_Close_Stream(t *testing.T) {
	ci.Parallel(t)

	// start server
	s1, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s1.RPC)

	policyNsGood := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob})
	tokenNsFoo := mock.CreatePolicyAndToken(t, s1.State(), 1006, "valid", policyNsGood)

	req := structs.EventStreamRequest{
		Topics: map[structs.Topic][]string{"Job": {"*"}},
		QueryOptions: structs.QueryOptions{
			Region:    s1.Region(),
			Namespace: structs.DefaultNamespace,
			AuthToken: tokenNsFoo.SecretID,
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

	job := mock.Job()
	jobEvent := structs.JobEvent{
		Job: job,
	}

	// send req
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	require.Nil(t, encoder.Encode(req))

	// publish some events
	publisher.Publish(&structs.Events{Index: uint64(1), Events: []structs.Event{{Topic: structs.TopicJob, Payload: jobEvent}}})
	publisher.Publish(&structs.Events{Index: uint64(2), Events: []structs.Event{{Topic: structs.TopicJob, Payload: jobEvent}}})

	// RPC to delete token
	aclDelReq := &structs.ACLTokenDeleteRequest{
		AccessorIDs: []string{tokenNsFoo.AccessorID},
		WriteRequest: structs.WriteRequest{
			Region:    s1.Region(),
			Namespace: structs.DefaultNamespace,
			AuthToken: root.SecretID,
		},
	}
	var aclResp structs.GenericResponse

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	codec := rpcClient(t, s1)
	errChStream := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				errChStream <- ctx.Err()
				return
			case err := <-errCh:
				errChStream <- err
				return
			case msg := <-streamMsg:
				if msg.Error == nil {
					// received a valid event, make RPC to delete token
					// continue trying for error
					continue
				}

				errChStream <- msg.Error
				return
			}
		}
	}()

	// Delete the token used to create the stream
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.DeleteTokens", aclDelReq, &aclResp))
	timeout := time.After(5 * time.Second)
OUTER:
	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for event stream")
		case err := <-errCh:
			t.Fatal(err)
		case err := <-errChStream:
			// Success
			require.Contains(t, err.Error(), stream.ErrSubscriptionClosed.Error())
			break OUTER
		}
	}
}

// TestEventStream_ACLTokenExpiry ensure a subscription does not receive events
// and is closed once the token has expired.
func TestEventStream_ACLTokenExpiry(t *testing.T) {
	ci.Parallel(t)

	// Start our test server and wait until we have a leader.
	testServer, _, testServerCleanup := TestACLServer(t, nil)
	defer testServerCleanup()
	testutil.WaitForLeader(t, testServer.RPC)

	// Create and upsert and ACL token which has a short expiry set.
	aclTokenWithExpiry := mock.ACLManagementToken()
	aclTokenWithExpiry.ExpirationTime = pointer.Of(time.Now().Add(2 * time.Second))

	must.NoError(t, testServer.fsm.State().UpsertACLTokens(
		structs.MsgTypeTestSetup, 10, []*structs.ACLToken{aclTokenWithExpiry}))

	req := structs.EventStreamRequest{
		Topics: map[structs.Topic][]string{"Job": {"*"}},
		QueryOptions: structs.QueryOptions{
			Region:    testServer.Region(),
			Namespace: structs.DefaultNamespace,
			AuthToken: aclTokenWithExpiry.SecretID,
		},
	}

	handler, err := testServer.StreamingRpcHandler("Event.Stream")
	must.NoError(t, err)

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

	publisher, err := testServer.State().EventBroker()
	must.NoError(t, err)

	jobEvent := structs.JobEvent{
		Job: mock.Job(),
	}

	// send req
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	must.Nil(t, encoder.Encode(req))

	// publish some events
	publisher.Publish(&structs.Events{Index: uint64(1), Events: []structs.Event{{Topic: structs.TopicJob, Payload: jobEvent}}})
	publisher.Publish(&structs.Events{Index: uint64(2), Events: []structs.Event{{Topic: structs.TopicJob, Payload: jobEvent}}})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(4*time.Second))
	defer cancel()

	errChStream := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				errChStream <- ctx.Err()
				return
			case err := <-errCh:
				errChStream <- err
				return
			case msg := <-streamMsg:
				if msg.Error == nil {
					continue
				}

				errChStream <- msg.Error
				return
			}
		}
	}()

	// Generate a timeout for the test and for the expiry. The expiry timeout
	// is used to trigger an update which will close the subscription as the
	// event stream only reacts to change in state.
	testTimeout := time.After(4 * time.Second)
	expiryTimeout := time.After(time.Until(*aclTokenWithExpiry.ExpirationTime))

	for {
		select {
		case <-testTimeout:
			t.Fatal("timeout waiting for event stream to close")
		case err := <-errCh:
			t.Fatal(err)
		case <-expiryTimeout:
			publisher.Publish(&structs.Events{Index: uint64(1), Events: []structs.Event{{Topic: structs.TopicJob, Payload: jobEvent}}})
		case err := <-errChStream:
			// Success
			must.StrContains(t, err.Error(), "ACL token expired")
			return
		}
	}
}
