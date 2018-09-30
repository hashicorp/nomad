package utils

import (
	"fmt"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/drivers"
	"golang.org/x/net/context"
)

var (
	// DefaultSendEventTimeout is the timeout used when publishing events to consumers
	DefaultSendEventTimeout = 2 * time.Second
)

// Eventer is a utility to control broadcast of TaskEvents to multiple consumers.
// It also implements the TaskEvents func in the DriverPlugin interface so that
// it can be embedded in a implementing driver struct.
type Eventer struct {
	consumersLock sync.RWMutex

	// events is a channel were events to be broadcasted are sent
	events chan *drivers.TaskEvent

	// consumers is a slice of eventConsumers to broadcast events to.
	// access is gaurded by consumersLock RWMutex
	consumers []*eventConsumer

	// ctx to allow control of event loop shutdown
	ctx context.Context

	// done tracks if the event loop has stopped due to the ctx being done
	done bool

	logger hclog.Logger
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
	for {
		select {
		case <-e.ctx.Done():
			e.done = true
			close(e.events)
			return
		case event := <-e.events:
			e.consumersLock.RLock()
			for _, consumer := range e.consumers {
				consumer.send(event)
			}
			e.consumersLock.RUnlock()
		}
	}
}

type eventConsumer struct {
	timeout time.Duration
	ctx     context.Context
	ch      chan *drivers.TaskEvent
	logger  hclog.Logger
}

func (c *eventConsumer) send(event *drivers.TaskEvent) {
	select {
	case <-time.After(c.timeout):
		c.logger.Warn("timeout sending event", "task_id", event.TaskID, "message", event.Message)
	case <-c.ctx.Done():
	case c.ch <- event:
	}
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
	go e.handleConsumer(consumer)
	return consumer.ch, nil
}

func (e *Eventer) handleConsumer(consumer *eventConsumer) {
	// wait for consumer or eventer ctx to finish
	select {
	case <-consumer.ctx.Done():
	case <-e.ctx.Done():
	}
	e.consumersLock.Lock()
	defer e.consumersLock.Unlock()
	defer close(consumer.ch)

	filtered := e.consumers[:0]
	for _, c := range e.consumers {
		if c != consumer {
			filtered = append(filtered, c)
		}
	}
	e.consumers = filtered
}

// EmitEvent can be used to broadcast a new event
func (e *Eventer) EmitEvent(event *drivers.TaskEvent) error {
	if e.done {
		return fmt.Errorf("error sending event, context canceled")
	}

	e.events <- event
	return nil
}
