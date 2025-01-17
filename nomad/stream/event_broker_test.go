// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/stretchr/testify/require"
)

func TestEventBroker_PublishChangesAndSubscribe(t *testing.T) {
	ci.Parallel(t)

	subscription := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": {"sub-key"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher, err := NewEventBroker(ctx, EventBrokerCfg{EventBufferSize: 100})
	require.NoError(t, err)

	sub, err := publisher.Subscribe(subscription)
	require.NoError(t, err)
	eventCh := consumeSubscription(ctx, sub)

	// Now subscriber should block waiting for updates
	assertNoResult(t, eventCh)

	events := []structs.Event{{
		Index:   1,
		Topic:   "Test",
		Key:     "sub-key",
		Payload: "sample payload",
	}}
	publisher.Publish(&structs.Events{Index: 1, Events: events})

	// Subscriber should see the published event
	result := nextResult(t, eventCh)
	require.NoError(t, result.Err)
	expected := []structs.Event{{Payload: "sample payload", Key: "sub-key", Topic: "Test", Index: 1}}
	require.Equal(t, expected, result.Events)

	// Now subscriber should block waiting for updates
	assertNoResult(t, eventCh)

	// Publish a second event
	events = []structs.Event{{
		Index:   2,
		Topic:   "Test",
		Key:     "sub-key",
		Payload: "sample payload 2",
	}}
	publisher.Publish(&structs.Events{Index: 2, Events: events})

	result = nextResult(t, eventCh)
	require.NoError(t, result.Err)
	expected = []structs.Event{{Payload: "sample payload 2", Key: "sub-key", Topic: "Test", Index: 2}}
	require.Equal(t, expected, result.Events)
}

func TestEventBroker_ShutdownClosesSubscriptions(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher, err := NewEventBroker(ctx, EventBrokerCfg{})
	require.NoError(t, err)

	sub1, err := publisher.Subscribe(&SubscribeRequest{})
	require.NoError(t, err)
	defer sub1.Unsubscribe()

	sub2, err := publisher.Subscribe(&SubscribeRequest{})
	require.NoError(t, err)
	defer sub2.Unsubscribe()

	cancel() // Shutdown

	err = consumeSub(context.Background(), sub1)
	require.Equal(t, err, ErrSubscriptionClosed)

	_, err = sub2.Next(context.Background())
	require.Equal(t, err, ErrSubscriptionClosed)
}

// TestEventBroker_EmptyReqToken_DistinctSubscriptions tests subscription
// hanlding behavior when ACLs are disabled (request Token is empty).
// Subscriptions are mapped by their request token.  when that token is empty,
// the subscriptions should still be handled independently of each other when
// unssubscribing.
func TestEventBroker_EmptyReqToken_DistinctSubscriptions(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher, err := NewEventBroker(ctx, EventBrokerCfg{})
	require.NoError(t, err)

	// first subscription, empty token
	sub1, err := publisher.Subscribe(&SubscribeRequest{})
	require.NoError(t, err)
	defer sub1.Unsubscribe()

	// second subscription, empty token
	sub2, err := publisher.Subscribe(&SubscribeRequest{})
	require.NoError(t, err)
	require.NotNil(t, sub2)

	sub1.Unsubscribe()

	require.Equal(t, subscriptionStateOpen, atomic.LoadUint32(&sub2.state))
}

func TestEventBroker_handleACLUpdates(t *testing.T) {
	ci.Parallel(t)

	secretID := "1234"

	testCases := []struct {
		name           string
		event          structs.Event
		shouldPassAuth bool
	}{
		{
			name: "token deleted",
			event: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenDeleted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
			shouldPassAuth: false, // shouldn't matter in token delete event
		},
		{
			name: "token updated - auth passes",
			event: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
			shouldPassAuth: false,
		},
		{
			name: "token updated - auth fails",
			event: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
			shouldPassAuth: true,
		},
		{
			name: "policy deleted",
			event: structs.Event{
				Topic: structs.TopicACLPolicy,
				Type:  structs.TypeACLPolicyDeleted,
				Payload: &structs.ACLPolicyEvent{
					ACLPolicy: &structs.ACLPolicy{
						Name: "some-policy",
					},
				},
			},
			shouldPassAuth: false,
		},
		{
			name: "policy updated - auth passes",
			event: structs.Event{
				Topic: structs.TopicACLPolicy,
				Type:  structs.TypeACLPolicyUpserted,
				Payload: &structs.ACLPolicyEvent{
					ACLPolicy: &structs.ACLPolicy{
						Name: "some-policy",
					},
				},
			},
			shouldPassAuth: true,
		},
		{
			name: "policy updated - auth fails",
			event: structs.Event{
				Topic: structs.TopicACLPolicy,
				Type:  structs.TypeACLTokenUpserted,
				Payload: &structs.ACLPolicyEvent{
					ACLPolicy: &structs.ACLPolicy{
						Name: "some-policy",
					},
				},
			},
			shouldPassAuth: false,
		},
		{
			name: "role delete",
			event: structs.Event{
				Topic: structs.TopicACLRole,
				Type:  structs.TypeACLRoleDeleted,
				Payload: &structs.ACLRoleStreamEvent{
					ACLRole: &structs.ACLRole{
						ID: "1234",
					},
				},
			},
			shouldPassAuth: false,
		},
		{
			name: "role updated - auth passes",
			event: structs.Event{
				Topic: structs.TopicACLRole,
				Type:  structs.TypeACLRoleUpserted,
				Payload: &structs.ACLRoleStreamEvent{
					ACLRole: &structs.ACLRole{
						ID: "1234",
					},
				},
			},
			shouldPassAuth: true,
		},
		{
			name: "role updated - auth fails",
			event: structs.Event{
				Topic: structs.TopicACLRole,
				Type:  structs.TypeACLRoleUpserted,
				Payload: &structs.ACLRoleStreamEvent{
					ACLRole: &structs.ACLRole{
						ID: "1234",
					},
				},
			},
			shouldPassAuth: false,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()

		publisher, err := NewEventBroker(ctx, EventBrokerCfg{})
		require.NoError(t, err)

		testSubReq := &SubscribeRequest{
			Topics: map[structs.Topic][]string{
				"*": {"*"},
			},
			Token: secretID,
			Authenticate: func() error {
				return nil
			},
		}

		sub, err := publisher.Subscribe(testSubReq)
		require.NoError(t, err)

		if !tc.shouldPassAuth {
			testSubReq.Authenticate = func() error {
				return structs.ErrPermissionDenied
			}
		}

		// publish the ACL event
		publisher.Publish(&structs.Events{Index: 100, Events: []structs.Event{tc.event}})

		_, err = sub.Next(ctx)

		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)

		// try to read another event
		_, err = sub.Next(ctx)
		if !tc.shouldPassAuth {
			require.ErrorIs(t, err, ErrSubscriptionClosed)
		} else {
			require.ErrorIs(t, err, context.DeadlineExceeded)
		}

		sub.Unsubscribe()
		cancel()
	}
}

func consumeSubscription(ctx context.Context, sub *Subscription) <-chan subNextResult {
	eventCh := make(chan subNextResult, 1)
	go func() {
		for {
			es, err := sub.Next(ctx)
			eventCh <- subNextResult{
				Events: es.Events,
				Err:    err,
			}
			if err != nil {
				return
			}
		}
	}()
	return eventCh
}

type subNextResult struct {
	Events []structs.Event
	Err    error
}

func nextResult(t *testing.T, eventCh <-chan subNextResult) subNextResult {
	t.Helper()
	select {
	case next := <-eventCh:
		return next
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("no event after 100ms")
	}
	return subNextResult{}
}

func assertNoResult(t *testing.T, eventCh <-chan subNextResult) {
	t.Helper()
	select {
	case next := <-eventCh:
		require.NoError(t, next.Err)
		require.Len(t, next.Events, 1)
		t.Fatalf("received unexpected event: %#v", next.Events[0].Payload)
	case <-time.After(100 * time.Millisecond):
	}
}

func consumeSub(ctx context.Context, sub *Subscription) error {
	for {
		_, err := sub.Next(ctx)
		if err != nil {
			return err
		}
	}
}
