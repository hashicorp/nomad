package nomad

import (
	"context"
	"errors"
	"fmt"
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

	logger hclog.Logger
}

// SinkDelegate is the interface needed for the SinkManger to interfact with
// parts of Nomad
type SinkDelegate interface {
	State() *state.StateStore
	getLeaderAcl() string
	Region() string
	RPC(method string, args interface{}, reply interface{}) error
}

// NewSinkManager builds a new SinkManager. It also creates ManagedSinks for
// all EventSinks in the state store
func NewSinkManager(ctx context.Context, delegate SinkDelegate, l hclog.Logger) *SinkManager {
	m := &SinkManager{
		ctx:                ctx,
		delegate:           delegate,
		updateSinkInterval: 30 * time.Second,
		sinkSubscriptions:  make(map[string]*ManagedSink),
		logger:             l.Named("sinks"),
	}

	return m
}

// establishManagedSinks creates and sets ManagedSinks for the Manager.
// establishManagedSinks should only be called from the SinkManager's Run
// loop.
func (m *SinkManager) establishManagedSinks() error {
	m.eventSinksWs = m.delegate.State().NewWatchSet()
	iter, err := m.delegate.State().EventSinks(m.eventSinksWs)
	if err != nil {
		return err
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		sink := raw.(*structs.EventSink)
		if _, ok := m.sinkSubscriptions[sink.ID]; !ok {
			ms, err := NewManagedSink(m.ctx, sink.ID, m.delegate.State, m.logger)
			if err != nil {
				return fmt.Errorf("creating managed sink: %w", err)
			}
			m.sinkSubscriptions[sink.ID] = ms
		}
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

	// run is used to run a ManagedSink. When the Sink errors the error will be
	// sent to the manager via errCh
	run := func(id string, ms *ManagedSink) {
		err := ms.run()

		select {
		case <-m.shutdownCh:
		case errCh <- SinkError{ID: id, Error: err}:
		}
	}

	if err := m.establishManagedSinks(); err != nil {
		return err
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
			return nil
		case <-m.shutdownCh:
			return nil
		case <-updateSinks.C:
			if err := m.updateSinkProgress(); err != nil {
				m.logger.Warn("unable to update sink progress", "error", err)
			}
		case err := <-m.ws().WatchCh(m.ctx):
			if err != nil {
				if err == context.Canceled {
					return nil
				}
				return err
			}

			// check for new sinks
			err = m.establishManagedSinks()
			if err != nil {
				return err
			}
			goto START

		case sinkErr := <-errCh:
			if sinkErr.Error == ErrEventSinkDeregistered {
				m.logger.Debug("sink deregistered, removing from manager", "sink", sinkErr.ID)
				// remove the sink from the manager
				delete(m.sinkSubscriptions, sinkErr.ID)
			} else {
				// TODO should this be an error log, should we do anything to re-run it
				m.logger.Warn("received error from managed event sink", "error", sinkErr.Error.Error())
			}
		}
	}

}

// updateSinkProgress sends an RPC to update the sinks with the latest progress.
// This should only be called from within the Manager's main Run loop.
func (m *SinkManager) updateSinkProgress() error {
	var updates []*structs.EventSink
	for _, ms := range m.sinkSubscriptions {
		progress := ms.GetLastSuccess()
		update := ms.Sink
		update.LatestIndex = progress

		updates = append(updates, update)
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

// refreshSinks checks for any new event sinks added to the state store. It
// adds new sinks as new ManagedSinks. This method must be externally
// synchronized in the SinkManger.Run main loop
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

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		sink := raw.(*structs.EventSink)
		if _, ok := m.sinkSubscriptions[sink.ID]; !ok {
			ms, err := NewManagedSink(m.ctx, sink.ID, m.delegate.State, m.logger)
			if err != nil {
				return err
			}
			m.sinkSubscriptions[sink.ID] = ms
		}
	}

	m.eventSinksWs = newSinkWs
	return nil
}

// ws returns the current newSinkWs used to listen for changes to the
// event sink table in the state store. ws() should only be called from Run
func (m *SinkManager) ws() memdb.WatchSet {
	return m.eventSinksWs
}

// ManagedSink maintains a subscription for a given EventSink. It is
// responsible for resubscribing and consuming the subscription, writing events
// to the ManagedSink's sinkWriter
type ManagedSink struct {
	// stopCtx is the passed in ctx used to signal that the ManagedSink should
	// stop running
	stopCtx context.Context

	// Sink is a copy of the state store EventSink
	// It must be a copy in order to be properly reloaded and notified via
	// its watchCh
	Sink *structs.EventSink

	// watchCh is used to watch for updates to the EventSink.
	watchCh <-chan error

	// subscription is the event stream subscription
	subscription *stream.Subscription

	// lastSuccess is the index of the last successfully sent index
	lastSuccess uint64

	// sinkWriter is an interface used to send events to their final destination
	sinkWriter stream.SinkWriter

	// stateFn returns the current server's StateStore
	stateFn func() *state.StateStore

	// sinkCtx is used to signal that the sink needs to be reloaded
	sinkCtx context.Context

	// sinkCancelFn cancels sinkCtx
	sinkCancelFn context.CancelFunc

	// mu coordinates access to running
	mu sync.Mutex

	// running specifies if the managed sink is running
	running bool

	logger hclog.Logger
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
		return nil, fmt.Errorf("error getting sink %s: %w", sinkID, err)
	}

	// TODO(drew) generate writer based off type
	writer, err := stream.NewWebhookSink(sink)
	if err != nil {
		return nil, fmt.Errorf("generating sink writer for sink %w", err)
	}

	sinkCtx, cancel := context.WithCancel(ctx)
	ms := &ManagedSink{
		stopCtx:      ctx,
		Sink:         sink,
		watchCh:      ws.WatchCh(sinkCtx),
		sinkWriter:   writer,
		sinkCancelFn: cancel,
		sinkCtx:      sinkCtx,
		stateFn:      stateFn,
		logger:       L.Named("managed_sink"), // TODO allow sink to name itself
	}

	return ms, nil
}

func (m *ManagedSink) Unsubscribe() {
	if m.subscription != nil {
		m.subscription.Unsubscribe()
	}
}

// run runs until the ManagedSink returns an non reloadable error or until the
// parent ctx is stopped.
func (m *ManagedSink) run() error {
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

	defer m.Unsubscribe()
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
			case err := <-m.watch():
				if err != nil {
					return
				}

				// WatchCh was triggered, reset the the WatchCh and potentially
				// cancel the currentSinkCtx to reload changes
				m.resetSink()
			}
		}
	}()

START:
	// Subscribe to event broker and establish SinkWriter
	err := m.subscribe()
	if err != nil {
		return err
	}

	currentSinkCtx := m.sinkCtx
	for {
		events, err := m.subscription.Next(currentSinkCtx)
		if err != nil {
			// Shutting down, exit gracefully
			if m.stopCtx.Err() != nil {
				return m.stopCtx.Err()
			}

			m.logger.Debug("received error from managed sink subscription, reloading sink", "error", err)
			goto START
		}

		// Send the events to the writer with stopCtx to cancel if manager is
		// shutting down
		err = m.sinkWriter.Send(m.stopCtx, &events)
		if err != nil {
			if strings.Contains(err.Error(), context.Canceled.Error()) {
				// if the context is canceled continue and let the subscription
				// error checking handle if we should reload or exit
				continue
			}
			m.logger.Warn("Failed to send event to sink", "sink", m.Sink.ID, "error", err)
			continue
		} else {
			// Update the last successful index sent
			atomic.StoreUint64(&m.lastSuccess, events.Index)
		}
	}
}

// resetSink resets the managed sinks watchCh after a change has been made
// to the event sink. It returns whether or not the subscription and sink
// should be reloaded.
func (m *ManagedSink) resetSink() {
	currentSubCancel := m.sinkCancelFn

	// Fetch the Event Sink
	ws := m.stateFn().NewWatchSet()
	sink, err := m.stateFn().EventSinkByID(ws, m.Sink.ID)
	if err != nil {
		// Log the error, continue to set the sink to nil
		m.logger.Error("error querying for latest event sink", "sink", m.Sink.ID, "error", err)
	}

	// Set our Sink to the new one with the corresponding watchCh
	oldSink := m.Sink
	m.Sink = sink
	m.watchCh = ws.WatchCh(m.stopCtx)

	// If the sink has been deregistered or if the sink has changed in a meaningful
	// way cancel the current subscription.
	if m.Sink == nil || !m.Sink.EqualSubscriptionValues(oldSink) {
		// Cancel the existing sinkCtx since we need to reload the subscription
		// and sink writer
		currentSubCancel()

		// Set the new sinkCtx and cancelFn
		sinkCtx, cancel := context.WithCancel(m.stopCtx)
		m.sinkCtx = sinkCtx
		m.sinkCancelFn = cancel
	}

}

// subscribe starts a new subscription to send to the SinkWriter
func (m *ManagedSink) subscribe() error {
	// Sink has been deleted, stop
	if m.Sink == nil {
		return ErrEventSinkDeregistered
	}

	// Unsubscribe from the current subscription if there is one
	if m.subscription != nil {
		m.subscription.Unsubscribe()
	}

	// Generate the sink writer
	writer, err := stream.NewWebhookSink(m.Sink)
	if err != nil {
		return fmt.Errorf("generating sink writer for sink %w", err)
	}
	m.sinkWriter = writer

	// Set the starting index for the new subscription. The locally tracked
	// Index may be ahead of the one periodically sent to raft, use whichever
	// is greater to reduce duplicates
	startIndex := m.Sink.LatestIndex
	localIndex := atomic.LoadUint64(&m.lastSuccess)
	if localIndex > startIndex {
		startIndex = localIndex
	}

	// Resubscribe
	req := &stream.SubscribeRequest{
		Topics: m.Sink.Topics,
		Index:  startIndex,
	}

	broker, err := m.stateFn().EventBroker()
	if err != nil {
		return fmt.Errorf("event broker error %w", err)
	}

	sub, err := broker.Subscribe(req)
	if err != nil {
		return fmt.Errorf("unable to subscribe sink %w", err)
	}
	m.subscription = sub

	return nil
}

// Running specifies if the ManagedSink is currently running
func (m *ManagedSink) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *ManagedSink) watch() <-chan error {
	return m.watchCh
}

func (m *ManagedSink) GetLastSuccess() uint64 {
	return atomic.LoadUint64(&m.lastSuccess)
}

type SinkError struct {
	ID    string
	Error error
}
