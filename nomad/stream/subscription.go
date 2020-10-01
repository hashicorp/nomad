package stream

import (
	"context"
	"errors"
	"sync/atomic"
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

type Subscription struct {
	// state is accessed atomically 0 means open, 1 means closed with reload
	state uint32

	req *SubscribeRequest

	// currentItem stores the current buffer item we are on. It
	// is mutated by calls to Next.
	currentItem *bufferItem

	// forceClosed is closed when forceClose is called. It is used by
	// EventPublisher to cancel Next().
	forceClosed chan struct{}

	// unsub is a function set by EventPublisher that is called to free resources
	// when the subscription is no longer needed.
	// It must be safe to call the function from multiple goroutines and the function
	// must be idempotent.
	unsub func()
}

type SubscribeRequest struct {
	Token string
	Index uint64

	Topics map[Topic][]string
}

func newSubscription(req *SubscribeRequest, item *bufferItem, unsub func()) *Subscription {
	return &Subscription{
		forceClosed: make(chan struct{}),
		req:         req,
		currentItem: item,
		unsub:       unsub,
	}
}

func (s *Subscription) Next(ctx context.Context) (Events, error) {
	if atomic.LoadUint32(&s.state) == subscriptionStateClosed {
		return Events{}, ErrSubscriptionClosed
	}

	for {
		next, err := s.currentItem.Next(ctx, s.forceClosed)
		switch {
		case err != nil && atomic.LoadUint32(&s.state) == subscriptionStateClosed:
			return Events{}, ErrSubscriptionClosed
		case err != nil:
			return Events{}, err
		}
		s.currentItem = next

		events := filter(s.req, next.Events)
		if len(events) == 0 {
			continue
		}
		return Events{Index: next.Index, Events: events}, nil
	}
}

func (s *Subscription) NextNoBlock() ([]Event, error) {
	if atomic.LoadUint32(&s.state) == subscriptionStateClosed {
		return nil, ErrSubscriptionClosed
	}

	for {
		next := s.currentItem.NextNoBlock()
		if next == nil {
			return nil, nil
		}
		s.currentItem = next

		events := filter(s.req, next.Events)
		if len(events) == 0 {
			continue
		}
		return events, nil
	}
}

func (s *Subscription) forceClose() {
	swapped := atomic.CompareAndSwapUint32(&s.state, subscriptionStateOpen, subscriptionStateClosed)
	if swapped {
		close(s.forceClosed)
	}
}

func (s *Subscription) Unsubscribe() {
	s.unsub()
}

// filter events to only those that match a subscriptions topic/keys
func filter(req *SubscribeRequest, events []Event) []Event {
	if len(events) == 0 {
		return events
	}

	var count int
	for _, e := range events {
		_, allTopics := req.Topics[AllKeys]
		if _, ok := req.Topics[e.Topic]; ok || allTopics {
			var keys []string
			if allTopics {
				keys = req.Topics[AllKeys]
			} else {
				keys = req.Topics[e.Topic]
			}
			for _, k := range keys {
				if e.Key == k || k == AllKeys {
					count++
				}
			}
		}
	}

	// Only allocate a new slice if some events need to be filtered out
	switch count {
	case 0:
		return nil
	case len(events):
		return events
	}

	// Return filtered events
	result := make([]Event, 0, count)
	for _, e := range events {
		_, allTopics := req.Topics[AllKeys]
		if _, ok := req.Topics[e.Topic]; ok || allTopics {
			var keys []string
			if allTopics {
				keys = req.Topics[AllKeys]
			} else {
				keys = req.Topics[e.Topic]
			}
			for _, k := range keys {
				if e.Key == k || k == AllKeys {
					result = append(result, e)
				}
			}
		}
	}
	return result
}
