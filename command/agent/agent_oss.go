// +build !ent

package agent

import (
	"context"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/command/agent/event"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

type noOpEventer struct{}

// Ensure noOpEventer is an Eventer
var _ event.Eventer = &noOpEventer{}

func (e *noOpEventer) Event(ctx context.Context, eventType string, payload interface{}) error {
	return nil
}

func (e *noOpEventer) Enabled() bool {
	return false
}

func (e *noOpEventer) Reopen() error {
	return nil
}

func (e *noOpEventer) SetEnabled(enabled bool) {}

func (a *Agent) setupEnterpriseAgent(log hclog.Logger) error {
	// configure eventer
	a.eventer = &noOpEventer{}

	return nil
}

func (a *Agent) entReloadEventer(cfg *config.AuditConfig) error {
	return nil
}
