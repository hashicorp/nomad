package nomad

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ErrEventSinkDeregistered is used to inform the EventSink Manager that a sink
// has been deleted
var ErrEventSinkDeregistered error = errors.New("sink deregistered")

// SinkManager manages all of the registered event sinks. It runs each sink as
// a ManagedSink and starts new sinks when they are registered
type SinkManager struct {
	// ctx is the passed in parent context that is used to signal that the
	// SinkManager should stop
	ctx context.Context

	// broker is the event broker
	broker *stream.EventBroker

	// mu synchronizes access to sinkSubscriptions and newSinkWs
	mu                sync.Mutex
	sinkSubscriptions map[string]*ManagedSink
	newSinkWs         memdb.WatchSet

	// stateFn is a function that returns a pointer to the servers state store
	stateFn func() *state.StateStore

	L hclog.Logger
}

// NewSinkManager builds a new SinkManager. It also creates ManagedSinks for
// all EventSinks in the state store
func NewSinkManager(ctx context.Context, stateFn func() *state.StateStore, L hclog.Logger) (*SinkManager, error) {
	state := stateFn()
	if state == nil {
		return nil, fmt.Errorf("state store was nil")
	}

	broker, err := state.EventBroker()
	if err != nil {
		return nil, err
	}

	newSinkWs := memdb.NewWatchSet()
	newSinkWs.Add(state.AbandonCh())

	m := &SinkManager{
		stateFn:           stateFn,
		broker:            broker,
		ctx:               ctx,
		sinkSubscriptions: make(map[string]*ManagedSink),
		newSinkWs:         newSinkWs,
		L:                 L,
	}

	iter, err := state.EventSinks(newSinkWs)
	if err != nil {
		return nil, err
	}
	var sinkIDs []string
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		sink := raw.(*structs.EventSink)
		sinkIDs = append(sinkIDs, sink.ID)
	}

	for _, id := range sinkIDs {
		mSink, err := NewManagedSink(ctx, id, stateFn, L)
		if err != nil {
			return nil, fmt.Errorf("creating managed sink: %w", err)
		}
		m.sinkSubscriptions[id] = mSink
	}

	return m, nil
}

func (m *SinkManager) Run() error {
	errCh := make(chan SinkError)
	execute := func(id string, ms *ManagedSink) {
		err := ms.Run()
		select {
		case <-m.ctx.Done():
		case errCh <- SinkError{ID: id, Error: err}:
		}
	}

START:
	for id, ms := range m.sinkSubscriptions {
		sid, sinkSub := id, ms
		if !ms.Running() {
			go execute(sid, sinkSub)
		}
	}

	for {
		select {
		case <-m.ctx.Done():
			return m.ctx.Err()
		case err := <-m.NewSinkWs().WatchCh(m.ctx):
			if err != nil {
				return err
			}
			// check for new sinks
			err = m.refreshSinks()
			if err != nil {
				return err
			}
			goto START

		case sinkErr := <-errCh:
			if sinkErr.Error == ErrEventSinkDeregistered {
				m.L.Debug("sink deregistered, removing from manager", "sink", sinkErr.ID)
				m.removeSink(sinkErr.ID)
			} else {
				m.L.Warn("received error from managed event sink", "error", sinkErr.Error.Error())
			}
		}
	}

}

func (m *SinkManager) removeSink(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sinkSubscriptions, id)
}

func (m *SinkManager) refreshSinks() error {
	state := m.stateFn()
	if state == nil {
		return fmt.Errorf("unable to fetch state store")
	}

	newSinkWs := state.NewWatchSet()
	iter, err := state.EventSinks(newSinkWs)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		sink := raw.(*structs.EventSink)
		if _, ok := m.sinkSubscriptions[sink.ID]; !ok {
			ms, err := NewManagedSink(m.ctx, sink.ID, m.stateFn, m.L)
			if err != nil {
				return err
			}
			m.sinkSubscriptions[sink.ID] = ms
		}
	}

	m.newSinkWs = newSinkWs
	return nil
}

// NewSinkWs returns the current newSinkWs used to listen for changes to the
// event sink table in the state store
func (m *SinkManager) NewSinkWs() memdb.WatchSet {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.newSinkWs
}

// ManagedSink maintains a subscription for a given EventSink. It is
// responsible for resubscribing and consuming the subscription, writing events
// to the managedsink's SinkWriter
type ManagedSink struct {
	// stopCtx is the passed in ctx used to signal that the ManagedSink should
	// stop running
	stopCtx context.Context

	// Sink is the state store EventSink
	Sink *structs.EventSink

	// watchCh is used to watch for updates to the ManagedSink's Sink.
	watchCh <-chan error

	// doneReset is used to notify that the ManagedSink is done reloading
	// itself from a subscription or state store change
	doneReset chan struct{}

	// Subscription is the event stream Subscription
	Subscription *stream.Subscription

	// LastSuccess is the index of the last successfully sent index
	LastSuccess uint64

	// SinkWriter is an interface used to send events to their final destination
	SinkWriter stream.SinkWriter

	// stateFn returns the current server's StateStore
	stateFn func() *state.StateStore

	// broker is the current server's event broker
	broker *stream.EventBroker

	// sinkCtx is used to signal that the sink needs to be reloaded
	sinkCtx context.Context

	// cancelFn cancels sinkCtx
	cancelFn context.CancelFunc

	// mu coordinates access to running
	mu sync.Mutex

	// running specifies if the managed sink is running
	running bool

	l hclog.Logger
}

// NewManagedSink returns a new ManagedSink for a given sinkID. It queries the
// state store and subscribes the sink to the state stores event broker
func NewManagedSink(ctx context.Context, sinkID string, stateFn func() *state.StateStore, L hclog.Logger) (*ManagedSink, error) {
	state := stateFn()
	if state == nil {
		return nil, fmt.Errorf("unable to fetch state store")
	}

	if L == nil {
		return nil, fmt.Errorf("logger was nil")
	}

	ws := state.NewWatchSet()
	sink, err := state.EventSinkByID(ws, sinkID)
	if err != nil {
		return nil, fmt.Errorf("getting sink %s: %w", sinkID, err)
	}

	// TODO(drew) generate writer based off type
	writer, err := stream.NewWebhookSink(sink)
	if err != nil {
		return nil, fmt.Errorf("generating sink writer for sink %w", err)
	}
	broker, err := state.EventBroker()
	if err != nil {
		return nil, err
	}

	sinkCtx, cancel := context.WithCancel(ctx)
	ms := &ManagedSink{
		stopCtx:    ctx,
		Sink:       sink,
		watchCh:    ws.WatchCh(sinkCtx),
		doneReset:  make(chan struct{}),
		SinkWriter: writer,
		broker:     broker,
		cancelFn:   cancel,
		sinkCtx:    sinkCtx,
		stateFn:    stateFn,
		l:          L,
	}

	req := &stream.SubscribeRequest{
		Topics: ms.Sink.Topics,
	}

	sub, err := ms.broker.Subscribe(req)
	if err != nil {
		return nil, fmt.Errorf("unable to subscribe sink %w", err)
	}
	ms.Subscription = sub

	return ms, nil
}

// Run runs until the ManagedSink returns an non reloadable error or until the
// parent ctx is stopped.
func (m *ManagedSink) Run() error {
	m.mu.Lock()
	if m.running {
		return fmt.Errorf("managed sink already running")
	}
	m.running = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()

	defer m.Subscription.Unsubscribe()
	exitCh := make(chan struct{})
	defer close(exitCh)

	// Listen for changes to EventSink. If there is a change cancel our local
	// context to stop the subscription and reload with new changes.
	go func() {
		for {
			select {
			case <-exitCh:
				return
			case <-m.stopCtx.Done():
				return
			case err := <-m.WatchCh():
				if err != nil {
					return
				}

				// Cancel the subscription scoped context
				m.cancelFn()

				// wait until the reset was done
				select {
				case <-m.stopCtx.Done():
					return
				case <-m.doneReset:
				case <-exitCh:
				}
			}
		}
	}()

LOOP:
	for {
		events, err := m.Subscription.Next(m.sinkCtx)
		if err != nil {
			// Shutting down, exit gracefully
			if m.stopCtx.Err() != nil {
				return m.stopCtx.Err()
			}

			// Reloadable error, reload and restart
			if err == stream.ErrSubscriptionClosed || err == context.Canceled {
				if err := m.Reload(); err != nil {
					return err
				}
				goto LOOP
			}
			return err
		}

		err = m.SinkWriter.Send(m.sinkCtx, &events)
		if err != nil {
			if strings.Contains(err.Error(), context.Canceled.Error()) {
				continue
			}
			m.l.Warn("Failed to send event to sink", "sink", m.Sink.ID, "error", err)
			continue
		}
		// Update the last successful index sent
		atomic.StoreUint64(&m.LastSuccess, events.Index)
	}
}

// Reload reloads and resets a ManagedSink.
func (m *ManagedSink) Reload() error {
	// Exit if shutting down
	if err := m.stopCtx.Err(); err != nil {
		return err
	}

	// Unsubscribe incase we haven't yet
	m.Subscription.Unsubscribe()

	// Fetch our updated or changed event sink with a new watchset
	ws := memdb.NewWatchSet()
	ws.Add(m.stateFn().AbandonCh())
	sink, err := m.stateFn().EventSinkByID(ws, m.Sink.ID)
	if err != nil {
		return err
	}

	// Sink has been deleted, stop
	if sink == nil {
		return ErrEventSinkDeregistered
	}

	// Reconfigure the sink writer
	writer, err := stream.NewWebhookSink(sink)
	if err != nil {
		return fmt.Errorf("generating sink writer for sink %w", err)
	}

	// Reset values we are updating
	sinkCtx, cancel := context.WithCancel(m.stopCtx)
	m.sinkCtx = sinkCtx
	m.cancelFn = cancel
	m.SinkWriter = writer
	m.Sink = sink
	m.watchCh = ws.WatchCh(sinkCtx)

	// Resubscribe
	req := &stream.SubscribeRequest{
		Topics: m.Sink.Topics,
		Index:  atomic.LoadUint64(&m.LastSuccess),
	}

	sub, err := m.broker.Subscribe(req)
	if err != nil {
		return fmt.Errorf("unable to subscribe sink %w", err)
	}
	m.Subscription = sub

	// signal we are done reloading
	m.doneReset <- struct{}{}
	return nil
}

// Running specifies if the ManagedSink is currently running
func (m *ManagedSink) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *ManagedSink) WatchCh() <-chan error {
	return m.watchCh
}

type SinkError struct {
	ID    string
	Error error
}
