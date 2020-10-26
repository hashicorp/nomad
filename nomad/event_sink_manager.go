package nomad

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

	// updateSinkInterval is the interval to update event sink progress to raft
	updateSinkInterval time.Duration

	// mu synchronizes access to sinkSubscriptions and eventSinksWs, running
	// and shutdownCh
	mu sync.Mutex

	// sinkSubscriptions are the set of managed sinks that the manager is tracking
	sinkSubscriptions map[string]*ManagedSink

	// eventSinksWs is a watchset used to check for new event sinks added to the state store
	eventSinksWs memdb.WatchSet

	// running specifies if the manager is running
	running bool

	// shutdownCh is used to stop the manager from running
	shutdownCh chan struct{}

	// delegate is the interface needed to interact with State and RPCs
	delegate SinkDelegate

	L hclog.Logger
}

// SinkDelegate is the interface needed for the SinkManger to interfact with
// parts of Nomad
type SinkDelegate interface {
	State() *state.StateStore
	getLeaderAcl() string
	Region() string
	RPC(method string, args interface{}, reply interface{}) error
}

type serverDelegate struct {
	*Server
}

// NewSinkManager builds a new SinkManager. It also creates ManagedSinks for
// all EventSinks in the state store
func NewSinkManager(ctx context.Context, delegate SinkDelegate, l hclog.Logger) *SinkManager {
	m := &SinkManager{
		ctx:                ctx,
		delegate:           delegate,
		updateSinkInterval: 30 * time.Second,
		sinkSubscriptions:  make(map[string]*ManagedSink),
		L:                  l.Named("sink_manager"),
	}

	return m
}

// EstablishManagedSinks creates and sets ManagedSinks for the Manager
func (m *SinkManager) EstablishManagedSinks() error {
	m.eventSinksWs = m.delegate.State().NewWatchSet()

	iter, err := m.delegate.State().EventSinks(m.eventSinksWs)
	if err != nil {
		return err
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
		mSink, err := NewManagedSink(m.ctx, id, m.delegate.State, m.L)
		if err != nil {
			return fmt.Errorf("creating managed sink: %w", err)
		}
		m.sinkSubscriptions[id] = mSink
	}
	return nil
}

// Running specifies if the manager is currently running
func (m *SinkManager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// Stop stops the manager
func (m *SinkManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	close(m.shutdownCh)
	m.running = false
}

// Run is a long running function that starts all of the ManagedSinks.
func (m *SinkManager) Run() error {
	m.mu.Lock()
	m.running = true
	m.shutdownCh = make(chan struct{})
	m.mu.Unlock()

	errCh := make(chan SinkError)

	updateSinks := time.NewTicker(m.updateSinkInterval)
	defer updateSinks.Stop()

	run := func(id string, ms *ManagedSink) {
		err := ms.Run()
		select {
		case <-m.ctx.Done():
		case errCh <- SinkError{ID: id, Error: err}:
		}
	}

START:
	for id, ms := range m.sinkSubscriptions {
		sid, sinkSub := id, ms
		if !sinkSub.Running() {
			go run(sid, sinkSub)
		}
	}

	for {
		select {
		case <-m.ctx.Done():
			return m.ctx.Err()
		case <-m.shutdownCh:
			return nil
		case <-updateSinks.C:
			if err := m.updateSinkProgress(); err != nil {
				m.L.Warn("unable to update sink progress", "error", err)
			}
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

// updateSinkProgress sends an RPC to update the sinks with the latest progress
func (m *SinkManager) updateSinkProgress() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var updates []*structs.EventSink
	for _, ms := range m.sinkSubscriptions {
		progress := ms.GetLastSuccess()
		update := ms.Sink
		update.LatestIndex = progress

		updates = append(updates, &update)
	}

	req := structs.EventSinkProgressRequest{
		Sinks: updates,
		WriteRequest: structs.WriteRequest{
			Region:    m.delegate.Region(),
			AuthToken: m.delegate.getLeaderAcl(),
		},
	}

	var resp structs.GenericResponse
	if err := m.delegate.RPC("Event.UpdateSinks", &req, &resp); err != nil {
		return err
	}
	return nil
}

func (m *SinkManager) removeSink(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sinkSubscriptions, id)
}

// refreshSinks checks for any new event sinks added to the state store. It
// adds new sinks as new ManagedSinks
func (m *SinkManager) refreshSinks() error {
	state := m.delegate.State()
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
			ms, err := NewManagedSink(m.ctx, sink.ID, m.delegate.State, m.L)
			if err != nil {
				return err
			}
			m.sinkSubscriptions[sink.ID] = ms
		}
	}

	m.eventSinksWs = newSinkWs
	return nil
}

// NewSinkWs returns the current newSinkWs used to listen for changes to the
// event sink table in the state store
func (m *SinkManager) NewSinkWs() memdb.WatchSet {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.eventSinksWs
}

// ManagedSink maintains a subscription for a given EventSink. It is
// responsible for resubscribing and consuming the subscription, writing events
// to the managedsink's SinkWriter
type ManagedSink struct {
	// stopCtx is the passed in ctx used to signal that the ManagedSink should
	// stop running
	stopCtx context.Context

	// Sink is a copy of the state store EventSink
	// It must be a copy in order to be properly reloaded and notified via
	// its watchCh
	Sink structs.EventSink

	// watchCh is used to watch for updates to the EventSink.
	watchCh <-chan error

	// doneReset is used to notify that the ManagedSink is done reloading
	// itself from a subscription or state store change
	doneReset chan struct{}

	// Subscription is the event stream Subscription
	Subscription *stream.Subscription

	// lastSuccess is the index of the last successfully sent index
	lastSuccess uint64

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
		Sink:       *sink,
		watchCh:    ws.WatchCh(sinkCtx),
		doneReset:  make(chan struct{}),
		SinkWriter: writer,
		broker:     broker,
		cancelFn:   cancel,
		sinkCtx:    sinkCtx,
		stateFn:    stateFn,
		l:          L.Named("managed_sink"),
	}

	req := &stream.SubscribeRequest{
		Topics: ms.Sink.Topics,
		Index:  ms.Sink.LatestIndex,
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

	// Listen for changes to EventSink. If there is a change cancel the sink's
	// local context to stop the subscription and reload with new changes.
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

				// check if we should reload or just reset

				reload := m.ResetWatchSet()

				if !reload {
					continue
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
		atomic.StoreUint64(&m.lastSuccess, events.Index)
	}
}

// ResetWatchSet resets the managed sinks watchCh after a change has been made
// to the event sink. It returns whether or not the subscription and sink
// should be reloaded.
func (m *ManagedSink) ResetWatchSet() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	ws := m.stateFn().NewWatchSet()
	sink, err := m.stateFn().EventSinkByID(ws, m.Sink.ID)
	if err != nil {
		return true
	}

	if sink == nil {
		return true
	}

	// Update the watchCh
	m.watchCh = ws.WatchCh(m.sinkCtx)

	current := m.Sink

	// Only reload if something that affects the sink destination
	// or subscription
	if current.Address != sink.Address ||
		current.Type != sink.Type ||
		!reflect.DeepEqual(current.Topics, sink.Topics) {
		return true
	}

	return false
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
	m.Sink = *sink
	m.watchCh = ws.WatchCh(sinkCtx)

	// Resubscribe
	req := &stream.SubscribeRequest{
		Topics: m.Sink.Topics,
		Index:  atomic.LoadUint64(&m.lastSuccess),
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

func (m *ManagedSink) GetLastSuccess() uint64 {
	return atomic.LoadUint64(&m.lastSuccess)
}

type SinkError struct {
	ID    string
	Error error
}
