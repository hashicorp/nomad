package stream

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/stretchr/testify/require"
)

func TestEventBroker_PublishChangesAndSubscribe(t *testing.T) {
	subscription := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": {"sub-key"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventBroker(ctx, nil, EventBrokerCfg{EventBufferSize: 100})
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
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher := NewEventBroker(ctx, nil, EventBrokerCfg{})

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
// the subscriptions should still be handled indeppendtly of each other when
// unssubscribing.
func TestEventBroker_EmptyReqToken_DistinctSubscriptions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher := NewEventBroker(ctx, nil, EventBrokerCfg{})

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

func TestEventBroker_handleACLUpdates_tokendeleted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher := NewEventBroker(ctx, nil, EventBrokerCfg{})

	sub1, err := publisher.Subscribe(&SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"*"},
		},
		Token: "foo",
	})
	require.NoError(t, err)
	defer sub1.Unsubscribe()

	aclEvent := structs.Event{
		Topic: structs.TopicACLToken,
		Type:  structs.TypeACLTokenDeleted,
		Payload: structs.ACLTokenEvent{
			ACLToken: &structs.ACLToken{
				SecretID: "foo",
			},
		},
	}

	publisher.Publish(&structs.Events{Index: 100, Events: []structs.Event{aclEvent}})
	for {
		_, err := sub1.Next(ctx)
		if err == ErrSubscriptionClosed {
			break
		}
	}

	out, err := sub1.Next(ctx)
	require.Error(t, err)
	require.Equal(t, ErrSubscriptionClosed, err)
	require.Equal(t, structs.Events{}, out)
}

type fakeACLDelegate struct {
	tokenProvider ACLTokenProvider
}

func (d *fakeACLDelegate) TokenProvider() ACLTokenProvider {
	return d.tokenProvider
}

type fakeACLTokenProvider struct {
	policy    *structs.ACLPolicy
	policyErr error
	token     *structs.ACLToken
	tokenErr  error
}

func (p *fakeACLTokenProvider) ACLTokenBySecretID(ws memdb.WatchSet, secretID string) (*structs.ACLToken, error) {
	return p.token, p.tokenErr
}

func (p *fakeACLTokenProvider) ACLPolicyByName(ws memdb.WatchSet, policyName string) (*structs.ACLPolicy, error) {
	return p.policy, p.policyErr
}

func TestEventBroker_handleACLUpdates_policyupdated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	policy := &structs.ACLPolicy{
		Name:  "some-policy",
		Rules: mock.NodePolicy(acl.PolicyRead),
	}
	policy.SetHash()

	tokenProvider := &fakeACLTokenProvider{
		policy: policy,
		token: &structs.ACLToken{
			SecretID: "some-secret-id",
			Policies: []string{"some-policy"},
		},
	}

	aclDelegate := &fakeACLDelegate{
		tokenProvider: tokenProvider,
	}

	publisher := NewEventBroker(ctx, aclDelegate, EventBrokerCfg{})

	cases := []struct {
		policyBeforeRules string
		policyAfterRules  string
		topics            map[structs.Topic][]string
		desc              string
		event             structs.Event
		shouldUnsubscribe bool
	}{
		// {
		// 	desc:              "subscribed to deployments and removed access",
		// 	policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
		// 	policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{}),
		// 	shouldUnsubscribe: true,
		// 	event: structs.Event{
		// 		Topic: structs.TopicDeployment,
		// 		Type:  structs.TypeDeploymentUpdate,
		// 		Payload: structs.DeploymentEvent{
		// 			Deployment: &structs.Deployment{
		// 				ID: "some-id",
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	desc:              "subscribed to evals and removed access",
		// 	policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
		// 	policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{}),
		// 	shouldUnsubscribe: true,
		// 	event: structs.Event{
		// 		Topic: structs.TopicEval,
		// 		Type:  structs.TypeEvalUpdated,
		// 		Payload: structs.EvalEvent{
		// 			Eval: &structs.Evaluation{
		// 				ID: "some-id",
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	desc:              "subscribed to allocs and removed access",
		// 	policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
		// 	policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{}),
		// 	shouldUnsubscribe: true,
		// 	event: structs.Event{
		// 		Topic: structs.TopicAlloc,
		// 		Type:  structs.TypeAllocUpdated,
		// 		Payload: structs.AllocEvent{
		// 			Alloc: &structs.Allocation{
		// 				ID: "some-id",
		// 			},
		// 		},
		// 	},
		// },
		{
			desc:              "subscribed to nodes and removed access",
			policyBeforeRules: mock.NodePolicy(acl.PolicyRead),
			policyAfterRules:  mock.NodePolicy(acl.PolicyDeny),
			shouldUnsubscribe: true,
			event: structs.Event{
				Topic: structs.TopicNode,
				Type:  structs.TypeNodeRegistration,
				Payload: structs.NodeStreamEvent{
					Node: &structs.Node{
						ID: "some-id",
					},
				},
			},
		},
		// {
		// 	desc:              "subscribed to deployments and no access change",
		// 	policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
		// 	policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
		// 	shouldUnsubscribe: false,
		// 	event: structs.Event{
		// 		Topic: structs.TopicDeployment,
		// 		Type:  structs.TypeDeploymentUpdate,
		// 		Payload: structs.DeploymentEvent{
		// 			Deployment: &structs.Deployment{
		// 				ID: "some-id",
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	desc:              "subscribed to evals and no access change",
		// 	policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
		// 	policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
		// 	shouldUnsubscribe: false,
		// 	event: structs.Event{
		// 		Topic: structs.TopicEval,
		// 		Type:  structs.TypeEvalUpdated,
		// 		Payload: structs.EvalEvent{
		// 			Eval: &structs.Evaluation{
		// 				ID: "some-id",
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	desc:              "subscribed to allocs and no access change",
		// 	policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
		// 	policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
		// 	shouldUnsubscribe: false,
		// 	event: structs.Event{
		// 		Topic: structs.TopicAlloc,
		// 		Type:  structs.TypeAllocUpdated,
		// 		Payload: structs.AllocEvent{
		// 			Alloc: &structs.Allocation{
		// 				ID: "some-id",
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	desc:              "subscribed to nodes and no access change",
		// 	policyBeforeRules: mock.NodePolicy(acl.PolicyRead),
		// 	policyAfterRules:  mock.NodePolicy(acl.PolicyRead),
		// 	shouldUnsubscribe: false,
		// 	event: structs.Event{
		// 		Topic: structs.TopicNode,
		// 		Type:  structs.TypeNodeRegistration,
		// 		Payload: structs.NodeStreamEvent{
		// 			Node: &structs.Node{
		// 				ID: "some-id",
		// 			},
		// 		},
		// 	},
		// },
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			sub1, err := publisher.Subscribe(&SubscribeRequest{
				Topics: map[structs.Topic][]string{
					tc.event.Topic: {"*"},
				},
				Token: "some-secret-id",
			})
			require.NoError(t, err)
			publisher.Publish(&structs.Events{Index: 100, Events: []structs.Event{tc.event}})

			ctx, cancel := context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()
			_, err = sub1.Next(ctx)
			require.NoError(t, err)

			aclEvent := structs.Event{
				Topic: structs.TopicACLToken,
				Type:  structs.TypeACLTokenUpserted,
				Payload: structs.ACLTokenEvent{
					ACLToken: &structs.ACLToken{
						SecretID: "some-secret-id",
					},
				},
			}

			publisher.Publish(&structs.Events{Index: 101, Events: []structs.Event{aclEvent}})

			ctx, cancel = context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()
			_, err = sub1.Next(ctx)
			if tc.shouldUnsubscribe {
				require.Equal(t, ErrSubscriptionClosed, err)
			} else {
				require.NoError(t, err)
			}

			publisher.Publish(&structs.Events{Index: 102, Events: []structs.Event{tc.event}})

			ctx, cancel = context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()
			_, err = sub1.Next(ctx)
			if tc.shouldUnsubscribe {
				require.Equal(t, ErrSubscriptionClosed, err)
			} else {
				require.NoError(t, err)
			}
		})
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
