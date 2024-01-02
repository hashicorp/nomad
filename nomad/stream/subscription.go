// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// subscriptionStateOpen is the default state of a subscription. An open
	// subscription may receive new events.
	subscriptionStateOpen uint32 = 0

	// subscriptionStateClosed indicates that the subscription was closed, possibly
	// as a result of a change to an ACL token, and will not receive new events.
	// The subscriber must issue a new Subscribe request.
	subscriptionStateClosed uint32 = 1
)

// ErrSubscriptionClosed is a error signalling the subscription has been
// closed. The client should Unsubscribe, then re-Subscribe.
var ErrSubscriptionClosed = errors.New("subscription closed by server, client should resubscribe")
var ErrACLInvalid = errors.New("Provided ACL token is invalid for requested topics")

type Subscription struct {
	// state must be accessed atomically 0 means open, 1 means closed with reload
	state uint32

	req *SubscribeRequest

	// currentItem stores the current buffer item we are on. It
	// is mutated by calls to Next.
	currentItem *bufferItem

	// forceClosed is closed when forceClose is called. It is used by
	// EventBroker to cancel Next().
	forceClosed chan struct{}

	// unsub is a function set by EventBroker that is called to free resources
	// when the subscription is no longer needed.
	// It must be safe to call the function from multiple goroutines and the function
	// must be idempotent.
	unsub func()
}

type SubscribeRequest struct {
	Token     string
	Index     uint64
	Namespace string

	Topics map[structs.Topic][]string

	// StartExactlyAtIndex specifies if a subscription needs to
	// start exactly at the requested Index. If set to false,
	// the closest index in the buffer will be returned if there is not
	// an exact match
	StartExactlyAtIndex bool
}

func newSubscription(req *SubscribeRequest, item *bufferItem, unsub func()) *Subscription {
	return &Subscription{
		forceClosed: make(chan struct{}),
		req:         req,
		currentItem: item,
		unsub:       unsub,
	}
}

func (s *Subscription) Next(ctx context.Context) (structs.Events, error) {
	if atomic.LoadUint32(&s.state) == subscriptionStateClosed {
		return structs.Events{}, ErrSubscriptionClosed
	}

	for {
		next, err := s.currentItem.Next(ctx, s.forceClosed)
		switch {
		case err != nil && atomic.LoadUint32(&s.state) == subscriptionStateClosed:
			return structs.Events{}, ErrSubscriptionClosed
		case err != nil:
			return structs.Events{}, err
		}
		s.currentItem = next

		events := filter(s.req, next.Events.Events)
		if len(events) == 0 {
			continue
		}
		return structs.Events{Index: next.Events.Index, Events: events}, nil
	}
}

func (s *Subscription) NextNoBlock() ([]structs.Event, error) {
	if atomic.LoadUint32(&s.state) == subscriptionStateClosed {
		return nil, ErrSubscriptionClosed
	}

	for {
		next := s.currentItem.NextNoBlock()
		if next == nil {
			return nil, nil
		}
		s.currentItem = next

		events := filter(s.req, next.Events.Events)
		if len(events) == 0 {
			continue
		}
		return events, nil
	}
}

func (s *Subscription) Unsubscribe() {
	s.unsub()
}

// filter events to only those that match a subscriptions topic/keys/namespace
func filter(req *SubscribeRequest, events []structs.Event) []structs.Event {
	if len(events) == 0 {
		return nil
	}

	allTopicKeys := req.Topics[structs.TopicAll]

	// Return all events if subscribed to all namespaces and all topics
	if req.Namespace == "*" && len(allTopicKeys) == 1 && allTopicKeys[0] == string(structs.TopicAll) {
		return events
	}

	var result []structs.Event

	for _, event := range events {
		if req.Namespace != "*" && event.Namespace != "" && event.Namespace != req.Namespace {
			continue
		}

		// *[*] always matches
		if len(allTopicKeys) == 1 && allTopicKeys[0] == string(structs.TopicAll) {
			result = append(result, event)
			continue
		}

		keys := allTopicKeys

		if topicKeys, ok := req.Topics[event.Topic]; ok {
			keys = append(keys, topicKeys...)
		}

		if len(keys) == 1 && keys[0] == string(structs.TopicAll) {
			result = append(result, event)
			continue
		}

		for _, key := range keys {
			if eventMatchesKey(event, key) {
				result = append(result, event)
				continue
			}
		}
	}

	return result
}

func eventMatchesKey(event structs.Event, key string) bool {
	if event.Key == key {
		return true
	}

	for _, fk := range event.FilterKeys {
		if fk == key {
			return true
		}
	}

	return false
}
