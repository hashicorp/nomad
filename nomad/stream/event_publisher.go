package stream

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

const (
	DefaultTTL = 1 * time.Hour
)

type EventPublisherCfg struct {
	EventBufferSize int64
	EventBufferTTL  time.Duration
	Logger          hclog.Logger
}

type EventPublisher struct {
	// lock protects the eventbuffer
	lock sync.Mutex

	// eventBuf stores a configurable amount of events in memory
	eventBuf *eventBuffer

	// pruneTick is the duration to periodically prune events from the event
	// buffer. Defaults to 5s
	pruneTick time.Duration

	logger hclog.Logger

	subscriptions *subscriptions

	// publishCh is used to send messages from an active txn to a goroutine which
	// publishes events, so that publishing can happen asynchronously from
	// the Commit call in the FSM hot path.
	publishCh chan changeEvents
}

type subscriptions struct {
	// lock for byToken. If both subscription.lock and EventPublisher.lock need
	// to be held, EventPublisher.lock MUST always be acquired first.
	lock sync.RWMutex

	// byToken is an mapping of active Subscriptions indexed by a token and
	// a pointer to the request.
	// When the token is modified all subscriptions under that token will be
	// reloaded.
	// A subscription may be unsubscribed by using the pointer to the request.
	byToken map[string]map[*SubscribeRequest]*Subscription
}

func NewEventPublisher(ctx context.Context, cfg EventPublisherCfg) *EventPublisher {
	if cfg.EventBufferTTL == 0 {
		cfg.EventBufferTTL = 1 * time.Hour
	}

	if cfg.Logger == nil {
		cfg.Logger = hclog.NewNullLogger()
	}

	buffer := newEventBuffer(cfg.EventBufferSize, cfg.EventBufferTTL)
	e := &EventPublisher{
		logger:    cfg.Logger.Named("event_publisher"),
		eventBuf:  buffer,
		publishCh: make(chan changeEvents, 64),
		subscriptions: &subscriptions{
			byToken: make(map[string]map[*SubscribeRequest]*Subscription),
		},
		pruneTick: 5 * time.Second,
	}

	go e.handleUpdates(ctx)
	go e.periodicPrune(ctx)

	return e
}

// Publish events to all subscribers of the event Topic.
func (e *EventPublisher) Publish(index uint64, events []Event) {
	if len(events) > 0 {
		e.publishCh <- changeEvents{index: index, events: events}
	}
}

// Subscribe returns a new Subscription for a given request.
func (e *EventPublisher) Subscribe(req *SubscribeRequest) (*Subscription, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	var head *bufferItem
	var offset int
	if req.Index != 0 {
		head, offset = e.eventBuf.StartAtClosest(req.Index)
	} else {
		head = e.eventBuf.Head()
	}
	if offset > 0 {
		e.logger.Warn("requested index no longer in buffer", "requsted", int(req.Index), "closest", int(head.Index))
	}

	// Empty head so that calling Next on sub
	start := newBufferItem(req.Index, []Event{})
	start.link.next.Store(head)
	close(start.link.ch)

	sub := newSubscription(req, start, e.subscriptions.unsubscribe(req))

	e.subscriptions.add(req, sub)
	return sub, nil
}

func (e *EventPublisher) handleUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			e.subscriptions.closeAll()
			return
		case update := <-e.publishCh:
			e.sendEvents(update)
		}
	}
}

func (e *EventPublisher) periodicPrune(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(e.pruneTick):
			e.lock.Lock()
			e.eventBuf.prune()
			e.lock.Unlock()
		}
	}
}

type changeEvents struct {
	index  uint64
	events []Event
}

// sendEvents sends the given events to the publishers event buffer.
func (e *EventPublisher) sendEvents(update changeEvents) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.eventBuf.Append(update.index, update.events)
}

func (s *subscriptions) add(req *SubscribeRequest, sub *Subscription) {
	s.lock.Lock()
	defer s.lock.Unlock()

	subsByToken, ok := s.byToken[req.Token]
	if !ok {
		subsByToken = make(map[*SubscribeRequest]*Subscription)
		s.byToken[req.Token] = subsByToken
	}
	subsByToken[req] = sub
}

func (s *subscriptions) closeSubscriptionsForTokens(tokenSecretIDs []string) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	for _, secretID := range tokenSecretIDs {
		if subs, ok := s.byToken[secretID]; ok {
			for _, sub := range subs {
				sub.forceClose()
			}
		}
	}
}

// unsubscribe returns a function that the subscription will call to remove
// itself from the subsByToken.
// This function is returned as a closure so that the caller doesn't need to keep
// track of the SubscriptionRequest, and can not accidentally call unsubscribe with the
// wrong pointer.
func (s *subscriptions) unsubscribe(req *SubscribeRequest) func() {
	return func() {
		s.lock.Lock()
		defer s.lock.Unlock()

		subsByToken, ok := s.byToken[req.Token]
		if !ok {
			return
		}
		delete(subsByToken, req)
		if len(subsByToken) == 0 {
			delete(s.byToken, req.Token)
		}
	}
}

func (s *subscriptions) closeAll() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, byRequest := range s.byToken {
		for _, sub := range byRequest {
			sub.forceClose()
		}
	}
}
