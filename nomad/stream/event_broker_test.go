// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"

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

	publisher, err := NewEventBroker(ctx, nil, EventBrokerCfg{EventBufferSize: 100})
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

	publisher, err := NewEventBroker(ctx, nil, EventBrokerCfg{})
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
// the subscriptions should still be handled indeppendtly of each other when
// unssubscribing.
func TestEventBroker_EmptyReqToken_DistinctSubscriptions(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher, err := NewEventBroker(ctx, nil, EventBrokerCfg{})
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

func TestEventBroker_handleACLUpdates_TokenDeleted(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher, err := NewEventBroker(ctx, nil, EventBrokerCfg{})
	require.NoError(t, err)

	sub1, err := publisher.Subscribe(&SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"*"},
		},
		Token: "foo",
	})
	require.NoError(t, err)
	defer sub1.Unsubscribe()

	aclEvent := structs.Event{
		Topic:   structs.TopicACLToken,
		Type:    structs.TypeACLTokenDeleted,
		Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: "foo"}),
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
	role      *structs.ACLRole
	roleErr   error
}

func (p *fakeACLTokenProvider) ACLTokenBySecretID(_ memdb.WatchSet, _ string) (*structs.ACLToken, error) {
	return p.token, p.tokenErr
}

func (p *fakeACLTokenProvider) ACLPolicyByName(_ memdb.WatchSet, _ string) (*structs.ACLPolicy, error) {
	return p.policy, p.policyErr
}

func (p *fakeACLTokenProvider) GetACLRoleByID(_ memdb.WatchSet, _ string) (*structs.ACLRole, error) {
	return p.role, p.roleErr
}

func TestEventBroker_handleACLUpdates_policyUpdated(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	secretID := "some-secret-id"
	cases := []struct {
		policyBeforeRules string
		policyAfterRules  string
		topics            map[structs.Topic][]string
		desc              string
		event             structs.Event
		policyEvent       structs.Event
		shouldUnsubscribe bool
		initialSubErr     bool
	}{
		{
			desc:              "subscribed to deployments and removed access",
			policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{}),
			shouldUnsubscribe: true,
			event: structs.Event{
				Topic: structs.TopicDeployment,
				Type:  structs.TypeDeploymentUpdate,
				Payload: structs.DeploymentEvent{
					Deployment: &structs.Deployment{
						ID: "some-id",
					},
				},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
		},
		{
			desc:              "subscribed to evals and removed access",
			policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{}),
			shouldUnsubscribe: true,
			event: structs.Event{
				Topic: structs.TopicEvaluation,
				Type:  structs.TypeEvalUpdated,
				Payload: structs.EvaluationEvent{
					Evaluation: &structs.Evaluation{
						ID: "some-id",
					},
				},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
		},
		{
			desc:              "subscribed to allocs and removed access",
			policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{}),
			shouldUnsubscribe: true,
			event: structs.Event{
				Topic: structs.TopicAllocation,
				Type:  structs.TypeAllocationUpdated,
				Payload: structs.AllocationEvent{
					Allocation: &structs.Allocation{
						ID: "some-id",
					},
				},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
		},
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
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
		},
		{
			desc:              "subscribed to evals in all namespaces and removed access",
			policyBeforeRules: mock.NamespacePolicy("*", "", []string{acl.NamespaceCapabilityReadJob}),
			policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			shouldUnsubscribe: true,
			event: structs.Event{
				Topic:     structs.TopicEvaluation,
				Type:      structs.TypeEvalUpdated,
				Namespace: "foo",
				Payload: structs.EvaluationEvent{
					Evaluation: &structs.Evaluation{
						ID: "some-id",
					},
				},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
		},
		{
			desc:              "subscribed to deployments and no access change",
			policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			shouldUnsubscribe: false,
			event: structs.Event{
				Topic: structs.TopicDeployment,
				Type:  structs.TypeDeploymentUpdate,
				Payload: structs.DeploymentEvent{
					Deployment: &structs.Deployment{
						ID: "some-id",
					},
				},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
		},
		{
			desc:              "subscribed to evals and no access change",
			policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			shouldUnsubscribe: false,
			event: structs.Event{
				Topic: structs.TopicEvaluation,
				Type:  structs.TypeEvalUpdated,
				Payload: structs.EvaluationEvent{
					Evaluation: &structs.Evaluation{
						ID: "some-id",
					},
				},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
		},
		{
			desc:              "subscribed to allocs and no access change",
			policyBeforeRules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			policyAfterRules:  mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			shouldUnsubscribe: false,
			event: structs.Event{
				Topic: structs.TopicAllocation,
				Type:  structs.TypeAllocationUpdated,
				Payload: structs.AllocationEvent{
					Allocation: &structs.Allocation{
						ID: "some-id",
					},
				},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
		},
		{
			desc:              "subscribed to nodes and no access change",
			policyBeforeRules: mock.NodePolicy(acl.PolicyRead),
			policyAfterRules:  mock.NodePolicy(acl.PolicyRead),
			shouldUnsubscribe: false,
			event: structs.Event{
				Topic: structs.TopicNode,
				Type:  structs.TypeNodeRegistration,
				Payload: structs.NodeStreamEvent{
					Node: &structs.Node{
						ID: "some-id",
					},
				},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
		},
		{
			desc:              "initial token insufficient privileges",
			initialSubErr:     true,
			policyBeforeRules: mock.NodePolicy(acl.PolicyDeny),
			event: structs.Event{
				Topic: structs.TopicNode,
				Type:  structs.TypeNodeRegistration,
				Payload: structs.NodeStreamEvent{
					Node: &structs.Node{
						ID: "some-id",
					},
				},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: secretID}),
			},
		},
		{
			desc:              "subscribed to nodes and policy change no change",
			policyBeforeRules: mock.NodePolicy(acl.PolicyRead),
			policyAfterRules:  mock.NodePolicy(acl.PolicyWrite),
			shouldUnsubscribe: false,
			event: structs.Event{
				Topic: structs.TopicNode,
				Type:  structs.TypeNodeRegistration,
				Payload: structs.NodeStreamEvent{
					Node: &structs.Node{
						ID: "some-id",
					},
				},
			},
			policyEvent: structs.Event{
				Topic: structs.TopicACLPolicy,
				Type:  structs.TypeACLPolicyUpserted,
				Payload: &structs.ACLPolicyEvent{
					ACLPolicy: &structs.ACLPolicy{
						Name: "some-policy",
					},
				},
			},
		},
		{
			desc:              "subscribed to nodes and policy change no access",
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
			policyEvent: structs.Event{
				Topic: structs.TopicACLPolicy,
				Type:  structs.TypeACLPolicyUpserted,
				Payload: &structs.ACLPolicyEvent{
					ACLPolicy: &structs.ACLPolicy{
						Name: "some-policy",
					},
				},
			},
		},
		{
			desc:              "subscribed to nodes policy deleted",
			policyBeforeRules: mock.NodePolicy(acl.PolicyRead),
			policyAfterRules:  "",
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
			policyEvent: structs.Event{
				Topic: structs.TopicACLPolicy,
				Type:  structs.TypeACLPolicyDeleted,
				Payload: &structs.ACLPolicyEvent{
					ACLPolicy: &structs.ACLPolicy{
						Name: "some-policy",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			policy := &structs.ACLPolicy{
				Name:  "some-policy",
				Rules: tc.policyBeforeRules,
			}
			policy.SetHash()

			tokenProvider := &fakeACLTokenProvider{
				policy: policy,
				token: &structs.ACLToken{
					SecretID: secretID,
					Policies: []string{policy.Name},
				},
			}

			aclDelegate := &fakeACLDelegate{
				tokenProvider: tokenProvider,
			}

			publisher, err := NewEventBroker(ctx, aclDelegate, EventBrokerCfg{})
			require.NoError(t, err)

			var ns string
			if tc.event.Namespace != "" {
				ns = tc.event.Namespace
			} else {
				ns = structs.DefaultNamespace
			}

			sub, expiryTime, err := publisher.SubscribeWithACLCheck(&SubscribeRequest{
				Topics: map[structs.Topic][]string{
					tc.event.Topic: {"*"},
				},
				Namespace: ns,
				Token:     secretID,
			})
			require.Nil(t, expiryTime)

			if tc.initialSubErr {
				require.Error(t, err)
				require.Nil(t, sub)
				return
			} else {
				require.NoError(t, err)
			}
			publisher.Publish(&structs.Events{Index: 100, Events: []structs.Event{tc.event}})

			ctx, cancel := context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()
			_, err = sub.Next(ctx)
			require.NoError(t, err)

			// Update the mock provider to use the after rules
			policyAfter := &structs.ACLPolicy{
				Name:        "some-new-policy",
				Rules:       tc.policyAfterRules,
				ModifyIndex: 101, // The ModifyIndex is used to caclulate the acl cache key
			}
			policyAfter.SetHash()

			tokenProvider.policy = policyAfter

			// Publish ACL event triggering subscription re-evaluation
			publisher.Publish(&structs.Events{Index: 101, Events: []structs.Event{tc.policyEvent}})
			// Publish another event
			publisher.Publish(&structs.Events{Index: 102, Events: []structs.Event{tc.event}})

			// If we are expecting to unsubscribe consume the subscription
			// until the expected error occurs.
			ctx, cancel = context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()
			if tc.shouldUnsubscribe {
				for {
					_, err = sub.Next(ctx)
					if err != nil {
						if err == context.DeadlineExceeded {
							require.Fail(t, err.Error())
						}
						if err == ErrSubscriptionClosed {
							break
						}
					}
				}
			} else {
				_, err = sub.Next(ctx)
				require.NoError(t, err)
			}

			publisher.Publish(&structs.Events{Index: 103, Events: []structs.Event{tc.event}})

			ctx, cancel = context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()
			_, err = sub.Next(ctx)
			if tc.shouldUnsubscribe {
				require.Equal(t, ErrSubscriptionClosed, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEventBroker_handleACLUpdates_roleUpdated(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Generate a UUID to use in all tests for the token secret ID and the role
	// ID.
	tokenSecretID := uuid.Generate()
	roleID := uuid.Generate()

	cases := []struct {
		name                  string
		aclPolicy             *structs.ACLPolicy
		roleBeforePolicyLinks []*structs.ACLRolePolicyLink
		roleAfterPolicyLinks  []*structs.ACLRolePolicyLink
		topics                map[structs.Topic][]string
		event                 structs.Event
		policyEvent           structs.Event
		shouldUnsubscribe     bool
		initialSubErr         bool
	}{
		{
			name: "deployments access policy link removed",
			aclPolicy: &structs.ACLPolicy{
				Name: "test-event-broker-acl-policy",
				Rules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{
					acl.NamespaceCapabilityReadJob},
				),
			},
			roleBeforePolicyLinks: []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			roleAfterPolicyLinks:  []*structs.ACLRolePolicyLink{},
			shouldUnsubscribe:     true,
			event: structs.Event{
				Topic:   structs.TopicDeployment,
				Type:    structs.TypeDeploymentUpdate,
				Payload: structs.DeploymentEvent{Deployment: &structs.Deployment{}},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: tokenSecretID}),
			},
		},
		{
			name: "evaluations access policy link removed",
			aclPolicy: &structs.ACLPolicy{
				Name: "test-event-broker-acl-policy",
				Rules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{
					acl.NamespaceCapabilityReadJob},
				),
			},
			roleBeforePolicyLinks: []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			roleAfterPolicyLinks:  []*structs.ACLRolePolicyLink{},
			shouldUnsubscribe:     true,
			event: structs.Event{
				Topic:   structs.TopicEvaluation,
				Type:    structs.TypeEvalUpdated,
				Payload: structs.EvaluationEvent{Evaluation: &structs.Evaluation{}},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: tokenSecretID}),
			},
		},
		{
			name: "allocations access policy link removed",
			aclPolicy: &structs.ACLPolicy{
				Name: "test-event-broker-acl-policy",
				Rules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{
					acl.NamespaceCapabilityReadJob},
				),
			},
			roleBeforePolicyLinks: []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			roleAfterPolicyLinks:  []*structs.ACLRolePolicyLink{},
			shouldUnsubscribe:     true,
			event: structs.Event{
				Topic:   structs.TopicAllocation,
				Type:    structs.TypeAllocationUpdated,
				Payload: structs.AllocationEvent{Allocation: &structs.Allocation{}},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: tokenSecretID}),
			},
		},
		{
			name: "nodes access policy link removed",
			aclPolicy: &structs.ACLPolicy{
				Name:  "test-event-broker-acl-policy",
				Rules: mock.NodePolicy(acl.PolicyRead),
			},
			roleBeforePolicyLinks: []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			roleAfterPolicyLinks:  []*structs.ACLRolePolicyLink{},
			shouldUnsubscribe:     true,
			event: structs.Event{
				Topic:   structs.TopicNode,
				Type:    structs.TypeNodeRegistration,
				Payload: structs.NodeStreamEvent{Node: &structs.Node{}},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: tokenSecretID}),
			},
		},
		{
			name: "deployment access no change",
			aclPolicy: &structs.ACLPolicy{
				Name: "test-event-broker-acl-policy",
				Rules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{
					acl.NamespaceCapabilityReadJob},
				),
			},
			roleBeforePolicyLinks: []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			roleAfterPolicyLinks:  []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			shouldUnsubscribe:     false,
			event: structs.Event{
				Topic:   structs.TopicDeployment,
				Type:    structs.TypeDeploymentUpdate,
				Payload: structs.DeploymentEvent{Deployment: &structs.Deployment{}},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: tokenSecretID}),
			},
		},
		{
			name: "evaluations access no change",
			aclPolicy: &structs.ACLPolicy{
				Name: "test-event-broker-acl-policy",
				Rules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{
					acl.NamespaceCapabilityReadJob},
				),
			},
			roleBeforePolicyLinks: []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			roleAfterPolicyLinks:  []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			shouldUnsubscribe:     false,
			event: structs.Event{
				Topic:   structs.TopicEvaluation,
				Type:    structs.TypeEvalUpdated,
				Payload: structs.EvaluationEvent{Evaluation: &structs.Evaluation{}},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: tokenSecretID}),
			},
		},
		{
			name: "allocations access no change",
			aclPolicy: &structs.ACLPolicy{
				Name: "test-event-broker-acl-policy",
				Rules: mock.NamespacePolicy(structs.DefaultNamespace, "", []string{
					acl.NamespaceCapabilityReadJob},
				),
			},
			roleBeforePolicyLinks: []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			roleAfterPolicyLinks:  []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			shouldUnsubscribe:     false,
			event: structs.Event{
				Topic:   structs.TopicAllocation,
				Type:    structs.TypeAllocationUpdated,
				Payload: structs.AllocationEvent{Allocation: &structs.Allocation{}},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: tokenSecretID}),
			},
		},
		{
			name: "nodes access no change",
			aclPolicy: &structs.ACLPolicy{
				Name:  "test-event-broker-acl-policy",
				Rules: mock.NodePolicy(acl.PolicyRead),
			},
			roleBeforePolicyLinks: []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			roleAfterPolicyLinks:  []*structs.ACLRolePolicyLink{{Name: "test-event-broker-acl-policy"}},
			shouldUnsubscribe:     false,
			event: structs.Event{
				Topic:   structs.TopicNode,
				Type:    structs.TypeNodeRegistration,
				Payload: structs.NodeStreamEvent{Node: &structs.Node{}},
			},
			policyEvent: structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: tokenSecretID}),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			// Build our fake token provider containing the relevant state
			// objects and add this to our new delegate. Keeping the token
			// provider setup separate means we can easily update its state.
			tokenProvider := &fakeACLTokenProvider{
				policy: tc.aclPolicy,
				token: &structs.ACLToken{
					SecretID: tokenSecretID,
					Roles:    []*structs.ACLTokenRoleLink{{ID: roleID}},
				},
				role: &structs.ACLRole{
					ID: uuid.Short(),
					Policies: []*structs.ACLRolePolicyLink{
						{Name: tc.aclPolicy.Name},
					},
				},
			}
			aclDelegate := &fakeACLDelegate{tokenProvider: tokenProvider}

			publisher, err := NewEventBroker(ctx, aclDelegate, EventBrokerCfg{})
			require.NoError(t, err)

			ns := structs.DefaultNamespace
			if tc.event.Namespace != "" {
				ns = tc.event.Namespace
			}

			sub, expiryTime, err := publisher.SubscribeWithACLCheck(&SubscribeRequest{
				Topics:    map[structs.Topic][]string{tc.event.Topic: {"*"}},
				Namespace: ns,
				Token:     tokenSecretID,
			})
			require.Nil(t, expiryTime)

			if tc.initialSubErr {
				require.Error(t, err)
				require.Nil(t, sub)
				return
			}

			require.NoError(t, err)
			publisher.Publish(&structs.Events{Index: 100, Events: []structs.Event{tc.event}})

			ctx, cancel := context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()
			_, err = sub.Next(ctx)
			require.NoError(t, err)

			// Overwrite the ACL role policy links with the updated version
			// which is expected to cause a change in the subscription.
			tokenProvider.role.Policies = tc.roleAfterPolicyLinks

			// Publish ACL event triggering subscription re-evaluation
			publisher.Publish(&structs.Events{Index: 101, Events: []structs.Event{tc.policyEvent}})
			publisher.Publish(&structs.Events{Index: 102, Events: []structs.Event{tc.event}})

			// If we are expecting to unsubscribe consume the subscription
			// until the expected error occurs.
			ctx, cancel = context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()
			if tc.shouldUnsubscribe {
				for {
					_, err = sub.Next(ctx)
					if err != nil {
						if err == context.DeadlineExceeded {
							require.Fail(t, err.Error())
						}
						if err == ErrSubscriptionClosed {
							break
						}
					}
				}
			} else {
				_, err = sub.Next(ctx)
				require.NoError(t, err)
			}

			publisher.Publish(&structs.Events{Index: 103, Events: []structs.Event{tc.event}})

			ctx, cancel = context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()
			_, err = sub.Next(ctx)
			if tc.shouldUnsubscribe {
				require.Equal(t, ErrSubscriptionClosed, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEventBroker_handleACLUpdates_tokenExpiry(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cases := []struct {
		name         string
		inputToken   *structs.ACLToken
		shouldExpire bool
	}{
		{
			name: "token does not expire",
			inputToken: &structs.ACLToken{
				AccessorID:     uuid.Generate(),
				SecretID:       uuid.Generate(),
				ExpirationTime: pointer.Of(time.Now().Add(100000 * time.Hour).UTC()),
				Type:           structs.ACLManagementToken,
			},
			shouldExpire: false,
		},
		{
			name: "token does expire",
			inputToken: &structs.ACLToken{
				AccessorID:     uuid.Generate(),
				SecretID:       uuid.Generate(),
				ExpirationTime: pointer.Of(time.Now().Add(100000 * time.Hour).UTC()),
				Type:           structs.ACLManagementToken,
			},
			shouldExpire: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			// Build our fake token provider containing the relevant state
			// objects and add this to our new delegate. Keeping the token
			// provider setup separate means we can easily update its state.
			tokenProvider := &fakeACLTokenProvider{token: tc.inputToken}
			aclDelegate := &fakeACLDelegate{tokenProvider: tokenProvider}

			publisher, err := NewEventBroker(ctx, aclDelegate, EventBrokerCfg{})
			require.NoError(t, err)

			fakeNodeEvent := structs.Event{
				Topic:   structs.TopicNode,
				Type:    structs.TypeNodeRegistration,
				Payload: structs.NodeStreamEvent{Node: &structs.Node{}},
			}

			fakeTokenEvent := structs.Event{
				Topic:   structs.TopicACLToken,
				Type:    structs.TypeACLTokenUpserted,
				Payload: structs.NewACLTokenEvent(&structs.ACLToken{SecretID: tc.inputToken.SecretID}),
			}

			sub, expiryTime, err := publisher.SubscribeWithACLCheck(&SubscribeRequest{
				Topics: map[structs.Topic][]string{structs.TopicAll: {"*"}},
				Token:  tc.inputToken.SecretID,
			})
			require.NoError(t, err)
			require.NotNil(t, sub)
			require.NotNil(t, expiryTime)

			// Publish an event and check that there is a new item in the
			// subscription queue.
			publisher.Publish(&structs.Events{Index: 100, Events: []structs.Event{fakeNodeEvent}})

			ctx, cancel := context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()
			_, err = sub.Next(ctx)
			require.NoError(t, err)

			// If the test states the token should expire, set the expiration
			// time to a previous time.
			if tc.shouldExpire {
				tokenProvider.token.ExpirationTime = pointer.Of(
					time.Date(1987, time.April, 13, 8, 3, 0, 0, time.UTC),
				)
			}

			// Publish some events to trigger re-evaluation of the subscription.
			publisher.Publish(&structs.Events{Index: 101, Events: []structs.Event{fakeTokenEvent}})
			publisher.Publish(&structs.Events{Index: 102, Events: []structs.Event{fakeNodeEvent}})

			// If we are expecting to unsubscribe consume the subscription
			// until the expected error occurs.
			ctx, cancel = context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
			defer cancel()

			if tc.shouldExpire {
				for {
					if _, err = sub.Next(ctx); err != nil {
						if err == context.DeadlineExceeded {
							require.Fail(t, err.Error())
						}
						if err == ErrSubscriptionClosed {
							break
						}
					}
				}
			} else {
				_, err = sub.Next(ctx)
				require.NoError(t, err)
			}
		})
	}
}

func TestEventBroker_NodePool_ACL(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testCases := []struct {
		name        string
		token       *structs.ACLToken
		policy      *structs.ACLPolicy
		expectedErr string
	}{
		{
			name: "management token",
			token: &structs.ACLToken{
				AccessorID: uuid.Generate(),
				SecretID:   uuid.Generate(),
				Type:       structs.ACLManagementToken,
			},
		},
		{
			name: "client token",
			token: &structs.ACLToken{
				AccessorID: uuid.Generate(),
				SecretID:   uuid.Generate(),
				Type:       structs.ACLClientToken,
			},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name: "node pool read",
			token: &structs.ACLToken{
				AccessorID: uuid.Generate(),
				SecretID:   uuid.Generate(),
				Type:       structs.ACLClientToken,
				Policies:   []string{"node-pool-read"},
			},
			policy: &structs.ACLPolicy{
				Name:  "node-pool-read",
				Rules: `node_pool "*" { policy = "read" }`,
			},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name: "node pool write",
			token: &structs.ACLToken{
				AccessorID: uuid.Generate(),
				SecretID:   uuid.Generate(),
				Type:       structs.ACLClientToken,
				Policies:   []string{"node-pool-write"},
			},
			policy: &structs.ACLPolicy{
				Name:  "node-pool-write",
				Rules: `node_pool "*" { policy = "write" }`,
			},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokenProvider := &fakeACLTokenProvider{token: tc.token, policy: tc.policy}
			aclDelegate := &fakeACLDelegate{tokenProvider: tokenProvider}

			publisher, err := NewEventBroker(ctx, aclDelegate, EventBrokerCfg{})
			must.NoError(t, err)

			_, _, err = publisher.SubscribeWithACLCheck(&SubscribeRequest{
				Topics: map[structs.Topic][]string{structs.TopicNodePool: {"*"}},
				Token:  tc.token.SecretID,
			})

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)
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
