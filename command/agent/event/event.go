package event

import (
	"context"
)

// Auditor describes the interface that must be implemented by an eventer.
type Auditor interface {
	// Emit an event to the auditor
	Event(ctx context.Context, eventType string, payload interface{}) error

	// Specifies if the auditor is enabled or not
	Enabled() bool

	// Reopen signals to auditor to reopen any files they have open.
	Reopen() error

	// SetEnabled sets the auditor to enabled or disabled.
	SetEnabled(enabled bool)

	// DeliveryEnforced returns whether or not delivery of an audit
	// log must be enforced
	DeliveryEnforced() bool
}
