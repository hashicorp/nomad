package utils

import (
	"sync"
	"time"

	"github.com/hashicorp/nomad/plugins/drivers/base"
	"golang.org/x/net/context"
)

var (
	//DefaultSendEventTimeout is the timeout used when publishing events to consumers
	DefaultSendEventTimeout = 2 * time.Second
)

// Eventer is a utility to control broadcast of TaskEvents to multiple consumers.
// It also implements the TaskStats func in the DriverPlugin interface so that
// it can be embedded in a implementing driver struct.
type Eventer struct {
	sync.RWMutex

	// events is a channel were events to be broadcasted are sent
	events chan *base.TaskEvent

	// streamers is a slice of consumers to broadcast events to
	// access is gaurded by RWMutex
	streamers []*eventStreamer

	// stop chan to allow control of event loop shutdown
	stop chan struct{}
}

// NewEventer returns an Eventer with a running event loop that can be stopped
// by closing the given stop channel
func NewEventer(stop chan struct{}) *Eventer {
	e := &Eventer{
		events: make(chan *base.TaskEvent),
		stop:   stop,
	}
	go e.eventLoop()
	return e
}

// eventLoop is the main logic which pulls events from the channel and broadcasts
// them to all consumers
func (e *Eventer) eventLoop() {
	for {
		select {
		case <-e.stop:
			for _, stream := range e.streamers {
				close(stream.ch)
			}
			return
		case event := <-e.events:
			e.RLock()
			for _, stream := range e.streamers {
				stream.send(event)
			}
			e.RUnlock()
		}
	}
}

type eventStreamer struct {
	timeout time.Duration
	ctx     context.Context
	ch      chan *base.TaskEvent
}

func (s *eventStreamer) send(event *base.TaskEvent) {
	select {
	case <-time.After(s.timeout):
	case <-s.ctx.Done():
	case s.ch <- event:
	}
}

func (e *Eventer) newStream(ctx context.Context) <-chan *base.TaskEvent {
	e.Lock()
	defer e.Unlock()

	stream := &eventStreamer{
		ch:      make(chan *base.TaskEvent),
		ctx:     ctx,
		timeout: DefaultSendEventTimeout,
	}
	e.streamers = append(e.streamers, stream)

	return stream.ch
}

// TaskEvents is an implementation of the DriverPlugin.TaskEvents function
func (e *Eventer) TaskEvents(ctx context.Context) (<-chan *base.TaskEvent, error) {
	stream := e.newStream(ctx)
	return stream, nil
}

// EmitEvent can be used to broadcast a new event
func (e *Eventer) EmitEvent(event *base.TaskEvent) {
	e.events <- event
}
