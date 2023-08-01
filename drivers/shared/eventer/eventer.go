package eventer

import (
	"context"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/plugins/drivers"
)

var (
	// DefaultSendEventTimeout is the timeout used when publishing events to consumers
	DefaultSendEventTimeout = 2 * time.Second

	// ConsumerGCInterval is the interval at which garbage collection of consumers
	// occures
	ConsumerGCInterval = time.Minute
)

// Eventer is a utility to control broadcast of TaskEvents to multiple consumers.
// It also implements the TaskEvents func in the DriverPlugin interface so that
// it can be embedded in a implementing driver struct.
type Eventer struct {

	// events is a channel were events to be broadcasted are sent
	// This channel is never closed, because it's lifetime is tied to the
	// life of the driver and closing creates some subtile race conditions
	// between closing it and emitting events.
	events chan *drivers.TaskEvent

	// consumers is a slice of eventConsumers to broadcast events to.
	// access is gaurded by consumersLock RWMutex
	consumers     []*eventConsumer
	consumersLock sync.RWMutex

	// ctx to allow control of event loop shutdown
	ctx context.Context

	logger hclog.Logger
}

type eventConsumer struct {
	timeout time.Duration
	ctx     context.Context
	ch      chan *drivers.TaskEvent
	logger  hclog.Logger
}

// NewEventer returns an Eventer with a running event loop that can be stopped
// by closing the given stop channel
func NewEventer(ctx context.Context, logger hclog.Logger) *Eventer {
	e := &Eventer{
		events: make(chan *drivers.TaskEvent),
		ctx:    ctx,
		logger: logger,
	}
	go e.eventLoop()
	return e
}

// eventLoop is the main logic which pulls events from the channel and broadcasts
// them to all consumers
func (e *Eventer) eventLoop() {
	timer, stop := helper.NewSafeTimer(ConsumerGCInterval)
	defer stop()

	for {
		timer.Reset(ConsumerGCInterval)

		select {
		case <-e.ctx.Done():
			e.logger.Trace("task event loop shutdown")
			return
		case event := <-e.events:
			e.iterateConsumers(event)
		case <-timer.C:
			e.gcConsumers()
		}
	}
}

// iterateConsumers will iterate through all consumers and broadcast the event,
// cleaning up any consumers that have closed their context
func (e *Eventer) iterateConsumers(event *drivers.TaskEvent) {
	e.consumersLock.Lock()
	filtered := e.consumers[:0]
	for _, consumer := range e.consumers {

		// prioritize checking if context is cancelled prior
		// to attempting to forwarding events
		// golang select evaluations aren't predictable
		if consumer.ctx.Err() != nil {
			close(consumer.ch)
			continue
		}

		select {
		case <-time.After(consumer.timeout):
			filtered = append(filtered, consumer)
			e.logger.Warn("timeout sending event", "task_id", event.TaskID, "message", event.Message)
		case <-consumer.ctx.Done():
			// consumer context finished, filtering it out of loop
			close(consumer.ch)
		case consumer.ch <- event:
			filtered = append(filtered, consumer)
		}
	}
	e.consumers = filtered
	e.consumersLock.Unlock()
}

func (e *Eventer) gcConsumers() {
	e.consumersLock.Lock()
	filtered := e.consumers[:0]
	for _, consumer := range e.consumers {
		select {
		case <-consumer.ctx.Done():
			// consumer context finished, filtering it out of loop
		default:
			filtered = append(filtered, consumer)
		}
	}
	e.consumers = filtered
	e.consumersLock.Unlock()
}

func (e *Eventer) newConsumer(ctx context.Context) *eventConsumer {
	e.consumersLock.Lock()
	defer e.consumersLock.Unlock()

	consumer := &eventConsumer{
		ch:      make(chan *drivers.TaskEvent),
		ctx:     ctx,
		timeout: DefaultSendEventTimeout,
		logger:  e.logger,
	}
	e.consumers = append(e.consumers, consumer)

	return consumer
}

// TaskEvents is an implementation of the DriverPlugin.TaskEvents function
func (e *Eventer) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	consumer := e.newConsumer(ctx)
	return consumer.ch, nil
}

// EmitEvent can be used to broadcast a new event
func (e *Eventer) EmitEvent(event *drivers.TaskEvent) error {

	select {
	case <-e.ctx.Done():
		return e.ctx.Err()
	case e.events <- event:
		if e.logger.IsTrace() {
			e.logger.Trace("emitting event", "event", event)
		}
	}
	return nil
}
