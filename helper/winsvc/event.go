// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

type WindowsEventId uint32

//go:generate stringer -trimprefix=Event -output strings_eventid.go -linecomment -type=WindowsEventId
const (
	EventUnknown         WindowsEventId = iota // unknown event
	EventServiceStarting                       // service starting
	EventServiceStarted                        // service started
	EventServiceStopped                        // service stopped
	EventLogMessage                            // log message
)

// NewEvent creates a new Event for the Windows Eventlog
func NewEvent(kind WindowsEventId, opts ...EventOption) Event {
	evt := &event{
		kind:  kind,
		level: EVENTLOG_LEVEL_INFO,
	}

	for _, fn := range opts {
		fn(evt)
	}

	return evt
}

type Event interface {
	Kind() WindowsEventId
	Message() string
	Level() EventLogLevel
}

type EventOption func(*event)

// WithEventMessage sets a custom message for the event
func WithEventMessage(msg string) EventOption {
	return func(e *event) {
		e.message = msg
	}
}

// WithEventLevel specifies the level used for the event
func WithEventLevel(level EventLogLevel) EventOption {
	return func(e *event) {
		e.level = level
	}
}

type event struct {
	kind    WindowsEventId
	message string
	level   EventLogLevel
}

func (e *event) Kind() WindowsEventId {
	return e.kind
}

func (e *event) Message() string {
	if e.message != "" {
		return e.message
	}

	return e.kind.String()
}

func (e *event) Level() EventLogLevel {
	return e.level
}
