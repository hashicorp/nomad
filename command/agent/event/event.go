package event

import (
	"context"
)

// Eventer describes the interface that must be implemented by an eventer.
type Eventer interface {
	// Emit and event
	Event(ctx context.Context, eventType string, payload interface{}) error
	// Specifies if the eventer is enabled or not
	Enabled() bool

	// Reopen signals to eventer to reopen any files they have open.
	Reopen() error

	// SetEnabled sets the eventer to enabled or disabled.
	SetEnabled(enabled bool)
}
