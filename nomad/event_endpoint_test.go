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

	"github.com/hashicorp/go-msgpack/v2/codec"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
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

func TestEventStream_validateNsOp(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	cases := []struct {
		Name        string
		Topics      map[structs.Topic][]string
		Namespace   string
		Policy      string
		Management  bool
		ExpectedErr error
	}{
		{
			Name: "read-job topics - correct ns",
			Topics: map[structs.Topic][]string{
				structs.TopicJob:        {"*"},
				structs.TopicEvaluation: {"*"},
				structs.TopicAllocation: {"*"},
				structs.TopicDeployment: {"*"},
				structs.TopicService:    {"*"},
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob}),
			Namespace:   "foo",
			Management:  false,
			ExpectedErr: nil,
		},
		{
			Name: "read-job topic - incorrect ns",
			Topics: map[structs.Topic][]string{
				structs.TopicJob: {"*"}, // good
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob}),
			Namespace:   "bar", // bad
			Management:  false,
			ExpectedErr: structs.ErrPermissionDenied,
		},
		{
			Name: "read all topics - correct policy",
			Topics: map[structs.Topic][]string{
				structs.TopicAll: {"*"}, // bad
			},
			Policy:      "",
			Namespace:   "*",
			Management:  true,
			ExpectedErr: nil,
		},
		{
			Name: "read all topics - incorrect policy",
			Topics: map[structs.Topic][]string{
				structs.TopicAll: {"*"}, // bad
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob}),
			Namespace:   "foo",
			Management:  false,
			ExpectedErr: structs.ErrPermissionDenied,
		},
		{
			Name: "read node - valid policy",
			Topics: map[structs.Topic][]string{
				structs.TopicNode: {"*"}, // bad
			},
			Policy:      mock.NodePolicy(acl.PolicyRead),
			Namespace:   "foo",
			Management:  false,
			ExpectedErr: nil,
		},
		{
			Name: "read node - invalid policy",
			Topics: map[structs.Topic][]string{
				structs.TopicEvaluation: {"*"}, // good
				structs.TopicNode:       {"*"}, // bad
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob}),
			Namespace:   "foo",
			Management:  false,
			ExpectedErr: structs.ErrPermissionDenied,
		},
		{
			Name: "read node pool - correct policy",
			Topics: map[structs.Topic][]string{
				structs.TopicNodePool: {"*"}, // bad
			},
			Policy:      "",
			Namespace:   "",
			Management:  true,
			ExpectedErr: nil,
		},
		{
			Name: "read node pool - incorrect policy",
			Topics: map[structs.Topic][]string{
				structs.TopicNodePool: {"*"}, // bad
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob}),
			Namespace:   "foo",
			Management:  false,
			ExpectedErr: structs.ErrPermissionDenied,
		},
		{
			Name: "read host volumes - correct policy and ns",
			Topics: map[structs.Topic][]string{
				structs.TopicHostVolume: {"*"},
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityHostVolumeRead}),
			Namespace:   "foo",
			Management:  false,
			ExpectedErr: nil,
		},
		{
			Name: "read host volumes - incorrect policy or ns",
			Topics: map[structs.Topic][]string{
				structs.TopicHostVolume: {"*"},
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob}),
			Namespace:   "foo",
			Management:  false,
			ExpectedErr: structs.ErrPermissionDenied,
		},
		{
			Name: "read csi volumes - correct policy and ns",
			Topics: map[structs.Topic][]string{
				structs.TopicCSIVolume: {"*"},
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityCSIReadVolume}),
			Namespace:   "foo",
			Management:  false,
			ExpectedErr: nil,
		},
		{
			Name: "read csi volumes - incorrect policy or ns",
			Topics: map[structs.Topic][]string{
				structs.TopicCSIVolume: {"*"},
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob}),
			Namespace:   "foo",
			Management:  false,
			ExpectedErr: structs.ErrPermissionDenied,
		},
		{
			Name: "read csi plugin - correct policy and ns",
			Topics: map[structs.Topic][]string{
				structs.TopicCSIPlugin: {"*"},
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob}),
			Namespace:   "foo",
			Management:  false,
			ExpectedErr: nil,
		},
		{
			Name: "read csi plugin - incorrect policy or ns",
			Topics: map[structs.Topic][]string{
				structs.TopicCSIPlugin: {"*"},
			},
			Policy:      mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob}),
			Namespace:   "bar",
			Management:  false,
			ExpectedErr: structs.ErrPermissionDenied,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {

			p, err := acl.Parse(tc.Policy)
			require.NoError(err)

			testACL, err := acl.NewACL(tc.Management, []*acl.Policy{p})
			require.NoError(err)

			err = validateNsOp(tc.Namespace, tc.Topics, testACL)
			require.Equal(tc.ExpectedErr, err)
		})
	}
}

func TestEventStream_validateACL(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s1.RPC)

	ns1 := mock.Namespace()

	err := s1.State().UpsertNamespaces(0, []*structs.Namespace{ns1})
	must.NoError(t, err)

	testEvent := &Event{srv: s1}

	t.Run("single namespace ACL errors on wildcard", func(t *testing.T) {
		policy, err := acl.Parse(mock.NamespacePolicy(ns1.Name, "", []string{acl.NamespaceCapabilityReadJob}))
		must.NoError(t, err)

		// does not contain policy for default NS
		testAcl, err := acl.NewACL(false, []*acl.Policy{policy})
		must.NoError(t, err)

		topics := map[structs.Topic][]string{
			structs.TopicJob: {"*"},
		}
		_, err = testEvent.validateACL("*", topics, testAcl)
		must.Error(t, err)
	})

	t.Run("all namespace ACL succeeds on wildcard", func(t *testing.T) {
		policy1, err := acl.Parse(mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityReadJob}))
		must.NoError(t, err)
		policy2, err := acl.Parse(mock.NamespacePolicy(ns1.Name, "", []string{acl.NamespaceCapabilityReadJob}))
		must.NoError(t, err)

		testAcl, err := acl.NewACL(false, []*acl.Policy{policy1, policy2})
		must.NoError(t, err)

		topics := map[structs.Topic][]string{
			structs.TopicJob: {"*"},
		}
		nses, err := testEvent.validateACL("*", topics, testAcl)
		must.NoError(t, err)
		must.Eq(t, nses, []string{"default", ns1.Name})
	})

	t.Run("single namespace ACL succeeds with correct NS", func(t *testing.T) {
		policy, err := acl.Parse(mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityReadJob}))
		must.NoError(t, err)

		testAcl, err := acl.NewACL(false, []*acl.Policy{policy})
		must.NoError(t, err)

		topics := map[structs.Topic][]string{
			structs.TopicJob: {"*"},
		}
		nses, err := testEvent.validateACL("default", topics, testAcl)
		must.NoError(t, err)
		must.Eq(t, nses, []string{"default"})
	})
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
